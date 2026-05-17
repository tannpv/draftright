package com.draftright.keyboard

class LanguageRegistry(private val packs: List<LanguagePack>) {

    init {
        require(packs.isNotEmpty()) { "LanguageRegistry needs at least one LanguagePack" }
    }

    val all: List<LanguagePack> = packs

    fun byId(id: String): LanguagePack =
        packs.firstOrNull { it.id == id }
            ?: throw NoSuchElementException("Unknown LanguagePack id: $id")

    fun byIdOrDefault(id: String): LanguagePack =
        packs.firstOrNull { it.id == id } ?: packs.first()

    fun next(currentId: String): LanguagePack {
        val idx = packs.indexOfFirst { it.id == currentId }
        if (idx < 0) return packs.first()
        return packs[(idx + 1) % packs.size]
    }
}
