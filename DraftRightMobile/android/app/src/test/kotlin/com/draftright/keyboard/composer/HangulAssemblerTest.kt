package com.draftright.keyboard.composer

import org.junit.Assert.assertEquals
import org.junit.Test

class HangulAssemblerTest {
    private fun a(s: String) = HangulAssembler.assemble(s)

    @Test fun `cho+jung makes a block`() = assertEquals("가", a("ㄱㅏ"))

    @Test fun `cho+jung+jong makes a CVC block`() = assertEquals("한", a("ㅎㅏㄴ"))

    @Test fun `annyeong`() = assertEquals("안녕", a("ㅇㅏㄴㄴㅕㅇ"))

    @Test fun `final moves to next initial when a vowel follows`() =
        assertEquals("가니", a("ㄱㅏㄴㅣ")) // 간 + ㅣ → 가 + 니

    @Test fun `compound vowel`() = assertEquals("과", a("ㄱㅗㅏ")) // ㅗ+ㅏ=ㅘ

    @Test fun `compound final`() = assertEquals("값", a("ㄱㅏㅂㅅ")) // ㅂ+ㅅ=ㅄ

    @Test fun `compound final then vowel splits`() =
        assertEquals("갑서", a("ㄱㅏㅂㅅㅓ")) // 값 + ㅓ → 갑 + 서

    @Test fun `multiple syllables`() = assertEquals("하나", a("ㅎㅏㄴㅏ"))

    @Test fun `lone initial consonant shown as-is`() = assertEquals("ㄱ", a("ㄱ"))

    @Test fun `trailing final attaches`() = assertEquals("간", a("ㄱㅏㄴ"))

    @Test fun `empty string`() = assertEquals("", a(""))
}
