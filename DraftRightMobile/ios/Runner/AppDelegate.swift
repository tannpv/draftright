import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  // Flutter 3.41 UIScene lifecycle: native plugins must be registered when
  // the implicit engine initialises. Without this hook, GeneratedPluginRegistrant
  // never runs, so plugin channels (shared_preferences, path_provider, …) never
  // connect — the app then silently fails wherever it touches a plugin. This is
  // what made onboarding "Get Started" appear unresponsive (App Store 2.1(a)
  // rejection of 2.3.3/57): SharedPreferences.getInstance() threw a channel
  // error, so the completion setState never ran. Matches the Flutter 3.41
  // scene-enabled template.
  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
  }
}
