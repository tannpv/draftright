# DraftRight Mobile — Android Keyboard Extension Implementation Plan (Plan 3 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Android keyboard extension (InputMethodService) that adds a DraftRight rewrite toolbar above the system keyboard.

**Architecture:** Android IME (Input Method Editor) using `InputMethodService`. The toolbar displays tone icons. On tone tap, it reads text via `InputConnection`, calls the OpenAI API, shows a diff bottom sheet within the IME view, and replaces text on confirm. Settings are read from SharedPreferences written by the Flutter main app.

**Tech Stack:** Kotlin, Android SDK, InputMethodService, OkHttp/HttpURLConnection, SharedPreferences

**Spec:** `docs/specs/2026-03-28-draftright-mobile-design.md`

---
