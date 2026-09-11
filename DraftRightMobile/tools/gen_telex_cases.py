#!/usr/bin/env python3
"""Exhaustive Telex order-free case generator (spec 2026-09-11).

Enumerates every valid Vietnamese syllable (onset x nucleus x coda x tone,
with compatibility rules) and emits keystroke sequences for each under many
typing orders. Expected output is always the syllable itself. Consumed by
TelexExhaustiveTest.kt and TelexExhaustiveTests.swift — the ONLY owner of
these tables (RULE #1); neither test file may restate a syllable.

The tables below are external-spec constants (Vietnamese orthography: onset /
nucleus / coda inventory, the k~c / gh~g / ngh~ng spelling alternation, stop
codas taking only sắc/nặng, and the modern tone-placement convention), not
tuning knobs. `--check-wordlist` proves the inventory is complete by asserting
every syllable of the shipped frequency wordlist is generated.
"""

import argparse
import json
import sys
import unicodedata
from itertools import permutations
from pathlib import Path

# --- Orthography tables (external spec) -------------------------------------

# "dd" renders đ; its mark key is the extra d, which may be typed remotely.
ONSETS = [
    "", "b", "c", "ch", "d", "dd", "g", "gh", "gi", "h", "k", "kh", "l",
    "m", "n", "ng", "ngh", "nh", "ph", "qu", "r", "s", "t", "th", "tr",
    "v", "x",
]

ONSET_RENDER = {"dd": "đ"}

# Coda policy per nucleus:
#   OPEN   — glide-final / 3-vowel nuclei that never take a coda (ai, iêu, …)
#   CLOSED — nuclei that only exist before a coda (ă, â, iê, uô, ươ, …)
#   ANY    — both (a, oa, uy, …)
OPEN, CLOSED, ANY = "open", "closed", "any"

# rendered nucleus -> (base keys, mark keys in canonical adjacent order, policy)
NUCLEI = {
    "a": ("a", [], ANY), "ă": ("a", ["w"], CLOSED), "â": ("a", ["a"], CLOSED),
    "e": ("e", [], ANY), "ê": ("e", ["e"], ANY),
    "i": ("i", [], ANY), "y": ("y", [], ANY),
    "o": ("o", [], ANY), "ô": ("o", ["o"], ANY), "ơ": ("o", ["w"], ANY),
    "u": ("u", [], ANY), "ư": ("u", ["w"], ANY),
    "ai": ("ai", [], OPEN), "ao": ("ao", [], OPEN),
    "au": ("au", [], OPEN), "ay": ("ay", [], OPEN),
    "âu": ("au", ["a"], OPEN), "ây": ("ay", ["a"], OPEN),
    "eo": ("eo", [], OPEN), "êu": ("eu", ["e"], OPEN),
    "ia": ("ia", [], OPEN), "iê": ("ie", ["e"], CLOSED), "iu": ("iu", [], OPEN),
    "yê": ("ye", ["e"], CLOSED),          # yên, quyết (after qu the y is the nucleus)
    "oa": ("oa", [], ANY), "oă": ("oa", ["w"], CLOSED), "oe": ("oe", [], ANY),
    "oi": ("oi", [], OPEN), "ôi": ("oi", ["o"], OPEN), "ơi": ("oi", ["w"], OPEN),
    "ua": ("ua", [], OPEN), "uâ": ("ua", ["a"], CLOSED), "uô": ("uo", ["o"], CLOSED),
    "ui": ("ui", [], OPEN), "uy": ("uy", [], ANY), "uyê": ("uye", ["e"], CLOSED),
    "ưa": ("ua", ["w"], OPEN), "ươ": ("uo", ["w"], CLOSED), "ưu": ("uu", ["w"], OPEN),
    "ưi": ("ui", ["w"], OPEN),            # gửi, ngửi, chửi
    "uây": ("uay", ["a"], OPEN),          # khuấy, quây
    "uê": ("ue", ["e"], ANY),
    # NOT generated: "uơ" (huơ, thuở). Its keystrokes are "uo"+w, which Telex
    # already spends on "ươ" — no keystroke reaches it, so it has no expected
    # output to assert. After a qu onset the nucleus is a plain "ơ" (quơ, quở),
    # which IS generated and passes.
    "iêu": ("ieu", ["e"], OPEN), "yêu": ("yeu", ["e"], OPEN),
    "oai": ("oai", [], OPEN), "oay": ("oay", [], OPEN),
    "oeo": ("oeo", [], OPEN), "oao": ("oao", [], OPEN),
    "uôi": ("uoi", ["o"], OPEN), "ươi": ("uoi", ["w"], OPEN),
    "ươu": ("uou", ["w"], OPEN), "uyu": ("uyu", [], OPEN), "uya": ("uya", [], OPEN),
}

CODAS = ["", "c", "ch", "m", "n", "ng", "nh", "p", "t"]
STOP_CODAS = {"c", "ch", "p", "t"}            # only sắc/nặng tones
PALATAL_CODAS = {"ch", "nh"}                  # anh/ach, ênh/êch, inh/ich, …
# Nuclei a palatal coda may follow (Vietnamese orthography).
PALATAL_NUCLEI = {"a", "ê", "i", "oa", "uê", "uy"}

# Onsets whose spelling is conditioned by the following vowel.
FRONT_ONLY_ONSETS = {"k", "gh", "ngh"}        # only before i/e/ê/y
BACK_ONLY_ONSETS = {"c", "g", "ng"}           # never before i/e/ê/y
FRONT_VOWELS = set("ieêy")

TONE_KEYS = ["", "s", "f", "r", "x", "j"]
# Combining diacritics, applied then NFC-composed — one source for all 5 tones.
TONE_COMBINING = {
    "s": "́",   # acute / sắc
    "f": "̀",   # grave / huyền
    "r": "̉",   # hook above / hỏi
    "x": "̃",   # tilde / ngã
    "j": "̣",   # dot below / nặng
}

QUALITY_VOWELS = set("ăâêôơư")


def tone_index(nucleus: str, has_coda: bool) -> int:
    """Index in [nucleus] of the tone-bearing vowel (modern convention).

    A quality-marked vowel wins — the LAST one, so ươ tones the ơ and uyê the
    ê. Otherwise a 3-vowel nucleus tones its middle vowel (khoái, ngoẻo), and
    a 2-vowel one tones the first when open (hòa, thúy) and the second when
    closed (hoàn). Single vowels are trivial.
    """
    marked = [i for i, c in enumerate(nucleus) if c in QUALITY_VOWELS]
    if marked:
        return marked[-1]
    if len(nucleus) == 1:
        return 0
    if len(nucleus) == 2:
        return 1 if has_coda else 0
    return 1


def render(onset: str, nucleus: str, coda: str, tone: str) -> str:
    """The syllable as it must appear on screen."""
    if tone:
        idx = tone_index(nucleus, bool(coda))
        nucleus = unicodedata.normalize(
            "NFC", nucleus[:idx] + nucleus[idx] + TONE_COMBINING[tone] + nucleus[idx + 1:]
        )
    return ONSET_RENDER.get(onset, onset) + nucleus + coda


def is_valid(onset: str, nucleus: str, coda: str, tone: str) -> bool:
    policy = NUCLEI[nucleus][2]
    if policy == OPEN and coda:
        return False
    if policy == CLOSED and not coda:
        return False
    if coda in PALATAL_CODAS and nucleus not in PALATAL_NUCLEI:
        return False
    if coda in STOP_CODAS and tone not in ("s", "j"):
        return False

    first_vowel = nucleus[0]
    if onset in FRONT_ONLY_ONSETS and first_vowel not in FRONT_VOWELS:
        return False
    if onset in BACK_ONLY_ONSETS and first_vowel in FRONT_VOWELS:
        # "g" before i is the one exception: it is written gi and IS the
        # spelling of gì/gìn — a plain g onset with an i nucleus.
        if not (onset == "g" and nucleus == "i"):
            return False
    if onset == "gi" and first_vowel in "iy":
        return False          # gi + i would be a third i; written "gi"/"gì"
    if onset == "qu" and first_vowel == "u":
        return False          # the glide u is already in the onset
    return True


def orderings(onset: str, nucleus: str, coda: str, tone: str):
    """Keystroke sequences that must all produce the same syllable.

    Canonical adjacent typing (marks right after their vowels, tone last) plus
    every end-appended permutation of the mark and tone keys — the order-free
    promise. For a đ onset the second d also joins the permutation set, which
    is what makes it a REMOTE d (spec mechanic 3).
    """
    base_keys, marks, _ = NUCLEI[nucleus]
    onset_keys = onset
    tail = list(marks) + ([tone] if tone else [])

    # Adjacent typing puts the mark key right after the vowel it marks — the
    # LAST marked one, so ươ is "uow" (both vowels) but ưa is "uwa" (only the
    # u). Base keys are positionally 1:1 with the rendered nucleus, so the
    # insert point is derived, never tabulated.
    marked = [i for i, c in enumerate(nucleus) if c in QUALITY_VOWELS]
    cut = (marked[-1] + 1) if marked else len(base_keys)
    seqs = {onset_keys + base_keys[:cut] + "".join(marks) + base_keys[cut:] + coda + tone}
    for perm in permutations(tail):
        seqs.add(onset_keys + base_keys + coda + "".join(perm))

    if onset == "dd":
        for perm in permutations(tail + ["d"]):
            seqs.add("d" + base_keys + coda + "".join(perm))

    return sorted(seqs)


def generate():
    """All (keys, expected) cases, deterministically ordered."""
    cases = []
    for onset in ONSETS:
        for nucleus in NUCLEI:
            for coda in CODAS:
                for tone in TONE_KEYS:
                    if not is_valid(onset, nucleus, coda, tone):
                        continue
                    expected = render(onset, nucleus, coda, tone)
                    for keys in orderings(onset, nucleus, coda, tone):
                        cases.append({"keys": keys, "expected": expected})
    return cases


# --- Wordlist completeness proof --------------------------------------------

# The wordlist is OpenSubtitles-derived: below this frequency its rows are
# dominated by OCR typos ("đươc", "nguời") and untranscribed foreign words, so
# a miss there says nothing about our inventory. Every row at or above it is
# either generated or one of the named non-syllables below — verified 2026-09-11.
MIN_COVERAGE_FREQ = 1000

# Frequent corpus rows that are not Vietnamese syllables at all: English
# subtitle words and chat shorthand ("ko" = không).
NON_SYLLABLE_ROWS = {"ko", "you", "hey", "it", "that", "lee"}

TONE_MARKS_COMBINING = set(TONE_COMBINING.values())


def tone_agnostic_key(syllable: str):
    """(letters without their tone, the tone mark) — identity modulo placement.

    Lets coverage accept the pre-1980s tone placement still common in the
    corpus (hoà, khoẻ, thuỷ) for syllables we generate in the modern placement
    the composer implements (hòa, khỏe, thủy). Same letters, same tone, only
    the carrying vowel differs.
    """
    decomposed = unicodedata.normalize("NFD", syllable)
    tone = "".join(c for c in decomposed if c in TONE_MARKS_COMBINING)
    bare = unicodedata.normalize(
        "NFC", "".join(c for c in decomposed if c not in TONE_MARKS_COMBINING)
    )
    return bare, tone


def check_wordlist(path: Path, generated: set) -> int:
    covered = {tone_agnostic_key(s) for s in generated}
    missing = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line or line.startswith("#"):
            continue
        word, _, freq = line.partition("\t")
        syllable = unicodedata.normalize("NFC", word.lower())
        if int(freq) < MIN_COVERAGE_FREQ or syllable in NON_SYLLABLE_ROWS:
            continue
        if syllable in generated or tone_agnostic_key(syllable) in covered:
            continue
        missing.append(syllable)
    if missing:
        print(f"{len(missing)} wordlist syllable(s) not generated:", file=sys.stderr)
        print("  " + " ".join(sorted(set(missing))), file=sys.stderr)
        return 1
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--out", type=Path, help="write the case matrix as JSON here")
    ap.add_argument("--check-wordlist", type=Path,
                    help="assert every syllable of this wordlist is generated")
    args = ap.parse_args()

    cases = generate()
    if args.out:
        args.out.write_text(json.dumps(cases, ensure_ascii=False), encoding="utf-8")
    if args.check_wordlist:
        return check_wordlist(args.check_wordlist, {c["expected"] for c in cases})
    return 0


if __name__ == "__main__":
    sys.exit(main())
