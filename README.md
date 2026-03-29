# DraftRight Mobile

AI-powered text rewriting keyboard for iOS and Android. Adds a rewrite toolbar above your system keyboard with tone options: Simple, More Natural, More Polished, Concise, Technical, Translate.

## Project Structure

- `lib/` — Flutter app (settings, onboarding, playground)
- `ios/DraftRightKeyboard/` — iOS keyboard extension (Swift) — Plan 2
- `android/keyboard/` — Android keyboard extension (Kotlin) — Plan 3

## Building

### Flutter App

```bash
flutter pub get
flutter run
```

### Run Tests

```bash
flutter test
```

## Setup

1. Install and open the app
2. Follow onboarding to enable the DraftRight keyboard
3. Enter your OpenAI API key (or configure a custom server) in Settings
4. Use the Playground to test rewrites
5. Switch to any messaging app — the DraftRight toolbar appears above your keyboard
