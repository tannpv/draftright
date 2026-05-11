# DraftRight Mobile

Flutter app + native keyboard extensions for iOS, Android, Windows, Linux.

## Architecture

```
lib/
├── main.dart              # Entry point — platform detection (mobile vs desktop)
├── screens/               # Login, Register, Settings, Onboarding, Playground, Subscription
├── services/
│   ├── auth_service.dart    # JWT token management, login/register/logout
│   ├── backend_client.dart  # POST /rewrite, GET /subscription
│   └── settings_service.dart # Backend URL, translation language
├── models/tone.dart       # Tone enum + display names
└── desktop/               # Windows/Linux: system tray, hotkey, floating panel

android/.../keyboard/      # Android IME (Kotlin)
├── DraftRightIME.kt         # InputMethodService + KeyboardActionListener
├── QwertyKeyboardView.kt    # Full QWERTY keyboard with 3 layers
├── ToolbarView.kt           # AI tone buttons toolbar
├── BackendClient.kt         # Calls backend /rewrite
├── SharedSettings.kt        # Reads JWT + backend URL from SharedPreferences
├── DiffSheetView.kt         # Side-by-side diff preview
└── Tone.kt                  # Tone enum

ios/DraftRightKeyboard/    # iOS keyboard extension (Swift)
├── KeyboardViewController.swift
├── ToolbarView.swift
├── BackendClient.swift
├── SharedSettings.swift
├── DiffSheetView.swift
└── Tone.swift
```

## Keyboard Extension ↔ Flutter App Communication

- Flutter writes JWT access token to SharedPreferences: key `flutter.draftright.accessToken`
- Flutter writes backend URL: key `flutter.draftright.backendUrl`
- Keyboard extensions read these from SharedPreferences (Android) / App Group UserDefaults (iOS)

## Bundle IDs

- **V1**: `com.draftright.draftright_mobile` (Android), `com.draftright.draftrightMobile` (iOS)
- **V2**: `com.draftright.draftright_mobile.v2` (Android), `com.draftright.draftrightMobile.v2` (iOS)

## Commands

```bash
flutter pub get
flutter run                          # Run on connected device
flutter build apk --debug           # Android debug build
flutter analyze                      # Dart analysis
cd android && ./gradlew assembleDebug  # Android build via Gradle
```

## Known Patterns

- Android keyboard uses programmatic views (no XML layouts)
- Key labels use emoji for special keys (shift ⬆, backspace ←, enter ↵, globe 🌐)
- Toolbar tone icons: ✎ 💬 ✨ ⊖ 🔧 🌐
- IME colors use explicit values (not theme attrs — unreliable in InputMethodService)
