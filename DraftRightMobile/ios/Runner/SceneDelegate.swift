import Flutter
import UIKit

/// The app uses the UIScene lifecycle (FlutterSceneDelegate auto-registers
/// plugins). The implicit-engine hook in AppDelegate never fires on this
/// path, so the App Group / keychain method channel is registered here,
/// against the live FlutterViewController's messenger, once the scene
/// connects. Without this the keyboard extension never receives the synced
/// token / backendUrl and its rewrite calls 401.
class SceneDelegate: FlutterSceneDelegate {

    private var appGroupChannel: FlutterMethodChannel?

    override func scene(_ scene: UIScene,
                        willConnectTo session: UISceneSession,
                        options connectionOptions: UIScene.ConnectionOptions) {
        super.scene(scene, willConnectTo: session, options: connectionOptions)
        registerAppGroupChannel(in: scene)
    }

    private func registerAppGroupChannel(in scene: UIScene, retriesLeft: Int = 10) {
        guard let windowScene = scene as? UIWindowScene,
              let messenger = windowScene.windows
                  .compactMap({ $0.rootViewController as? FlutterViewController })
                  .first?.binaryMessenger
        else {
            // FlutterViewController may not be attached yet — retry shortly.
            if retriesLeft > 0 {
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
                    self?.registerAppGroupChannel(in: scene, retriesLeft: retriesLeft - 1)
                }
            }
            return
        }

        let channel = FlutterMethodChannel(name: "com.draftright.v2/app_group",
                                           binaryMessenger: messenger)
        appGroupChannel = channel
        let defaults = UserDefaults(suiteName: "group.com.draftright.v2")
        assert(defaults != nil, "App Group group.com.draftright.v2 unavailable — check entitlements; keyboard sync will silently no-op")

        channel.setMethodCallHandler { call, result in
            let args = call.arguments as? [String: Any]
            let key = args?["key"] as? String
            switch call.method {
            case "set":
                guard let key else { result(invalidArgs()); return }
                if let value = args?["value"] as? String {
                    defaults?.set(value, forKey: key)
                } else {
                    defaults?.removeObject(forKey: key)
                }
                result(true)
            case "get":
                guard let key else { result(invalidArgs()); return }
                result(defaults?.string(forKey: key))
            case "setKeychain":
                guard let key else { result(invalidArgs()); return }
                result(SharedKeychain.set(key, args?["value"] as? String))
            case "getKeychain":
                guard let key else { result(invalidArgs()); return }
                result(SharedKeychain.get(key))
            case "deleteKeychain":
                guard let key else { result(invalidArgs()); return }
                result(SharedKeychain.delete(key))
            case "sharedPackDir":
                // App Group container path so downloaded IME language packs land
                // where the keyboard extension can read (mmap) them.
                let url = FileManager.default.containerURL(
                    forSecurityApplicationGroupIdentifier: "group.com.draftright.v2")
                result(url?.path)
            default:
                result(FlutterMethodNotImplemented)
            }
        }
    }
}

private func invalidArgs() -> FlutterError {
    FlutterError(code: "INVALID_ARGS", message: "key required", details: nil)
}
