//
//  Generated file. Do not edit.
//

import FlutterMacOS
import Foundation

import facebook_auth_desktop
import flutter_secure_storage_macos
import google_sign_in_ios
import hotkey_manager_macos
import screen_retriever
import shared_preferences_foundation
import system_tray
import window_manager

func RegisterGeneratedPlugins(registry: FlutterPluginRegistry) {
  FacebookAuthDesktopPlugin.register(with: registry.registrar(forPlugin: "FacebookAuthDesktopPlugin"))
  FlutterSecureStoragePlugin.register(with: registry.registrar(forPlugin: "FlutterSecureStoragePlugin"))
  FLTGoogleSignInPlugin.register(with: registry.registrar(forPlugin: "FLTGoogleSignInPlugin"))
  HotkeyManagerMacosPlugin.register(with: registry.registrar(forPlugin: "HotkeyManagerMacosPlugin"))
  ScreenRetrieverPlugin.register(with: registry.registrar(forPlugin: "ScreenRetrieverPlugin"))
  SharedPreferencesPlugin.register(with: registry.registrar(forPlugin: "SharedPreferencesPlugin"))
  SystemTrayPlugin.register(with: registry.registrar(forPlugin: "SystemTrayPlugin"))
  WindowManagerPlugin.register(with: registry.registrar(forPlugin: "WindowManagerPlugin"))
}
