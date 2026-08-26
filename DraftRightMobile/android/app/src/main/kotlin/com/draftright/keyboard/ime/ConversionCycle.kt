package com.draftright.keyboard.ime

/**
 * Pending-conversion cursor for JP/ZH space cycling (#207). Standard CJK IME:
 * the first space converts the reading to the top candidate as a *pending*
 * (still-marked) conversion; each further space cycles to the next candidate;
 * any other input confirms the current one.
 *
 * This holds only the candidate list + which one is selected — a pure, testable
 * state machine. The IME owns one instance and drives the marked-text / commit
 * side effects; keeping the cursor logic here means the wrap-around and
 * active/idle rules are unit-tested without an InputConnection.
 */
class ConversionCycle {

    private var candidates: List<String> = emptyList()
    private var index: Int = 0

    /** True once a conversion is pending (space was pressed on a reading). */
    val isActive: Boolean get() = candidates.isNotEmpty()

    /** Begin a pending conversion over [cands] (top candidate selected). No-op
     *  for an empty list, so callers can start() unconditionally. */
    fun start(cands: List<String>) {
        candidates = cands
        index = 0
    }

    /** The currently-selected candidate, or null when idle. */
    fun current(): String? = candidates.getOrNull(index)

    /** Advance to the next candidate (wraps after the last) and return it; null
     *  when idle. */
    fun advance(): String? {
        if (candidates.isEmpty()) return null
        index = (index + 1) % candidates.size
        return current()
    }

    /** Clear the pending conversion (after confirm/cancel). */
    fun reset() {
        candidates = emptyList()
        index = 0
    }
}
