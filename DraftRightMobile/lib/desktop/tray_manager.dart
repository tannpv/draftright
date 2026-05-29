import 'dart:io';
import 'package:flutter/material.dart';
import 'package:system_tray/system_tray.dart';
import 'package:window_manager/window_manager.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/settings_screen.dart';

class DraftRightTrayManager {
  final SystemTray _systemTray = SystemTray();

  VoidCallback? onSettingsRequested;
  VoidCallback? onQuitRequested;

  Future<void> init({
    required VoidCallback onSettings,
    required VoidCallback onQuit,
  }) async {
    onSettingsRequested = onSettings;
    onQuitRequested = onQuit;

    // Use a platform-appropriate icon path
    String iconPath = _getTrayIconPath();

    await _systemTray.initSystemTray(
      title: '',
      iconPath: iconPath,
      toolTip: 'DraftRight — AI text rewriting',
    );

    await _buildMenu();

    _systemTray.registerSystemTrayEventHandler((eventName) {
      if (eventName == kSystemTrayEventClick) {
        // Left-click on tray: show menu (Windows behavior)
        if (Platform.isWindows) {
          _systemTray.popUpContextMenu();
        }
      } else if (eventName == kSystemTrayEventRightClick) {
        _systemTray.popUpContextMenu();
      }
    });
  }

  String _getTrayIconPath() {
    if (Platform.isWindows) {
      return 'assets/tray_icon.ico';
    } else {
      // Linux — PNG works with most status notifier implementations
      return 'assets/tray_icon.png';
    }
  }

  Future<void> _buildMenu() async {
    final Menu menu = Menu();
    await menu.buildFrom([
      MenuItemLabel(
        label: 'DraftRight',
        enabled: false,
      ),
      MenuSeparator(),
      MenuItemLabel(
        label: 'Settings',
        onClicked: (_) => onSettingsRequested?.call(),
      ),
      MenuItemLabel(
        label: 'About',
        onClicked: (_) => _showAbout(),
      ),
      MenuSeparator(),
      MenuItemLabel(
        label: 'Quit',
        onClicked: (_) => onQuitRequested?.call(),
      ),
    ]);
    await _systemTray.setContextMenu(menu);
  }

  void _showAbout() {
    // Show a simple notification-style tooltip via tray title update
    // Full dialog would require the settings window to be open
    _systemTray.setTitle('DraftRight v1.0');
    Future.delayed(const Duration(seconds: 3), () {
      _systemTray.setTitle('');
    });
  }

  Future<void> dispose() async {
    await _systemTray.destroy();
  }
}

/// Standalone settings window shown when the user clicks Settings in the tray.
class DesktopSettingsWindow extends StatelessWidget {
  final SettingsService settings;
  const DesktopSettingsWindow({super.key, required this.settings});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider.value(
      value: settings,
      child: MaterialApp(
        title: 'DraftRight Settings',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
              seedColor: Colors.blue, brightness: Brightness.dark),
          useMaterial3: true,
        ),
        home: Scaffold(
          body: Column(
            children: [
              // Custom title bar with close button (frameless window)
              Container(
                color: Theme.of(context).colorScheme.surface,
                child: Row(
                  children: [
                    const Expanded(
                      child: Padding(
                        padding: EdgeInsets.all(12.0),
                        child: Text(
                          'DraftRight Settings',
                          style: TextStyle(fontWeight: FontWeight.bold),
                        ),
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close),
                      onPressed: () async {
                        await windowManager.hide();
                      },
                    ),
                  ],
                ),
              ),
              const Expanded(child: SettingsScreen()),
            ],
          ),
        ),
      ),
    );
  }
}
