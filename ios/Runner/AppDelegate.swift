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

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)

    // Get binary messenger from the plugin registry
    guard let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "AppGroupPlugin") else { return }
    let messenger = registrar.messenger()
    let channel = FlutterMethodChannel(name: "com.draftright.v2/app_group", binaryMessenger: messenger)
    let defaults = UserDefaults(suiteName: "group.com.draftright.v2")

    channel.setMethodCallHandler { (call, result) in
      switch call.method {
      case "set":
        if let args = call.arguments as? [String: Any],
           let key = args["key"] as? String {
          if let value = args["value"] as? String {
            defaults?.set(value, forKey: key)
          } else {
            defaults?.removeObject(forKey: key)
          }
          defaults?.synchronize()
          result(true)
        } else {
          result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
        }
      case "get":
        if let args = call.arguments as? [String: Any],
           let key = args["key"] as? String {
          result(defaults?.string(forKey: key))
        } else {
          result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
        }
      case "setKeychain":
        if let args = call.arguments as? [String: Any],
           let key = args["key"] as? String {
          let value = args["value"] as? String
          result(SharedKeychain.set(key, value))
        } else {
          result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
        }
      case "getKeychain":
        if let args = call.arguments as? [String: Any],
           let key = args["key"] as? String {
          result(SharedKeychain.get(key))
        } else {
          result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
        }
      case "deleteKeychain":
        if let args = call.arguments as? [String: Any],
           let key = args["key"] as? String {
          result(SharedKeychain.delete(key))
        } else {
          result(FlutterError(code: "INVALID_ARGS", message: "key required", details: nil))
        }
      default:
        result(FlutterMethodNotImplemented)
      }
    }
  }
}
