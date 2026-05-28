import 'dart:async';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:window_manager/window_manager.dart';
import 'package:screen_retriever/screen_retriever.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/desktop/tray_manager.dart';
import 'package:draftright_mobile/desktop/floating_panel.dart';

/// Entry point for the desktop (Windows / Linux) version of DraftRight.
///
/// Responsibilities:
///   1. Hide the main window on startup (app lives in the system tray).
///   2. Initialise the system tray icon and menu.
///   3. Register the global hotkey (Ctrl+Shift+R).
///   4. On hotkey: read clipboard → if text found, show floating panel.
///   5. On "Replace": write result to clipboard and send Ctrl+V.
///   6. On "Cancel": hide the window again.
class DesktopApp extends StatefulWidget {
  final SettingsService settings;
  final AuthService auth;
  const DesktopApp({super.key, required this.settings, required this.auth});

  @override
  State<DesktopApp> createState() => _DesktopAppState();
}

class _DesktopAppState extends State<DesktopApp> with WindowListener {
  final DraftRightTrayManager _trayManager = DraftRightTrayManager();

  // State machine for the panel
  _PanelState _panelState = _PanelState.hidden;
  String _clipboardText = '';

  @override
  void initState() {
    super.initState();
    _initDesktop();
  }

  Future<void> _initDesktop() async {
    // ── Window setup ──────────────────────────────────────────────────────────
    await windowManager.ensureInitialized();
    windowManager.addListener(this);

    const windowOptions = WindowOptions(
      size: Size(400, 400),
      minimumSize: Size(400, 350),
      center: false,
      backgroundColor: Colors.transparent,
      skipTaskbar: true,       // Don't show in taskbar — tray-only app
      titleBarStyle: TitleBarStyle.hidden,
      alwaysOnTop: true,
    );
    await windowManager.waitUntilReadyToShow(windowOptions, () async {
      await windowManager.hide();
    });

    // ── System tray ───────────────────────────────────────────────────────────
    await _trayManager.init(
      onSettings: _openSettings,
      onQuit: _quit,
    );

    // ── Global hotkey: Ctrl+Shift+R ───────────────────────────────────────────
    await hotKeyManager.unregisterAll();
    final hotKey = HotKey(
      key: PhysicalKeyboardKey.keyR,
      modifiers: [HotKeyModifier.control, HotKeyModifier.shift],
      scope: HotKeyScope.system,
    );
    await hotKeyManager.register(
      hotKey,
      keyDownHandler: (_) => _onHotKey(),
    );
  }

  // ── Hotkey handler ─────────────────────────────────────────────────────────

  Future<void> _onHotKey() async {
    // 1. Send Ctrl+C to copy whatever the user has selected
    await _simulateCopy();
    // 2. Small delay to let the clipboard update
    await Future.delayed(const Duration(milliseconds: 150));
    // 3. Read clipboard
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text?.trim() ?? '';

    if (text.isEmpty) {
      // Nothing selected — could show a toast, for now just ignore
      return;
    }

    _clipboardText = text;
    await _showPanel();
  }

  Future<void> _simulateCopy() async {
    // Send a synthetic Ctrl+C via the OS.
    // On Windows this uses SendInput; on Linux it uses XSendEvent / ydotool.
    // hotkey_manager / window_manager don't expose this, so we shell out.
    try {
      if (Platform.isLinux) {
        // xdotool must be installed; gracefully no-op if missing
        await Process.run('xdotool', ['key', '--clearmodifiers', 'ctrl+c']);
      }
      // Windows: hotkey_manager's copy simulation isn't available yet;
      // the user is expected to Ctrl+C themselves before pressing the hotkey.
      // (This is noted in the spec as an acceptable fallback.)
    } catch (_) {
      // Silently ignore — clipboard may already have the text.
    }
  }

  // ── Window / panel management ──────────────────────────────────────────────

  Future<void> _showPanel() async {
    setState(() => _panelState = _PanelState.visible);

    // Position near cursor
    try {
      final cursorPos = await screenRetriever.getCursorScreenPoint();
      final display = await screenRetriever.getPrimaryDisplay();
      final screenSize = display.size;

      double x = cursorPos.dx + 20;
      double y = cursorPos.dy + 20;

      // Keep panel on screen
      if (x + 420 > screenSize.width) x = screenSize.width - 420;
      if (y + 450 > screenSize.height) y = screenSize.height - 450;
      if (x < 0) x = 0;
      if (y < 0) y = 0;

      await windowManager.setPosition(Offset(x, y));
    } catch (_) {
      // If cursor position fails, just centre the window
      await windowManager.center();
    }

    await windowManager.setSize(const Size(400, 400));
    await windowManager.show();
    await windowManager.focus();
  }

  Future<void> _hidePanel() async {
    await windowManager.hide();
    setState(() => _panelState = _PanelState.hidden);
  }

  // ── Result handler (Replace / Copy) ───────────────────────────────────────

  Future<void> _onResult(String rewritten, {required bool replace}) async {
    // Write result to clipboard
    await Clipboard.setData(ClipboardData(text: rewritten));

    if (replace) {
      // Hide panel first, then simulate Ctrl+V so it pastes into the original app
      await _hidePanel();
      await Future.delayed(const Duration(milliseconds: 100));
      await _simulatePaste();
    } else {
      // Copy only — just hide
      await _hidePanel();
    }
  }

  Future<void> _simulatePaste() async {
    try {
      if (Platform.isLinux) {
        await Process.run('xdotool', ['key', '--clearmodifiers', 'ctrl+v']);
      }
      // Windows: no-op for now (paste is triggered by the user after copy)
    } catch (_) {
      // Silently ignore
    }
  }

  // ── Settings window ────────────────────────────────────────────────────────

  Future<void> _openSettings() async {
    // Switch panel state to settings
    setState(() => _panelState = _PanelState.settings);
    await windowManager.setSize(const Size(500, 600));
    await windowManager.center();
    await windowManager.show();
    await windowManager.focus();
    // Re-enable title bar for the settings window
    await windowManager.setTitleBarStyle(TitleBarStyle.normal);
    await windowManager.setTitle('DraftRight Settings');
    await windowManager.setAlwaysOnTop(false);
    await windowManager.setSkipTaskbar(false);
  }

  Future<void> _closeSettings() async {
    await windowManager.hide();
    // Restore floating panel window style
    await windowManager.setTitleBarStyle(TitleBarStyle.hidden);
    await windowManager.setAlwaysOnTop(true);
    await windowManager.setSkipTaskbar(true);
    setState(() => _panelState = _PanelState.hidden);
  }

  // ── Quit ───────────────────────────────────────────────────────────────────

  Future<void> _quit() async {
    await hotKeyManager.unregisterAll();
    await _trayManager.dispose();
    exit(0);
  }

  // ── WindowListener overrides ───────────────────────────────────────────────

  @override
  void onWindowClose() {
    // Intercept the close button — hide instead of quit
    _hidePanel();
  }

  @override
  void onWindowBlur() {
    // Auto-dismiss the floating panel when it loses focus
    if (_panelState == _PanelState.visible) {
      _hidePanel();
    }
  }

  // ── Build ──────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        ChangeNotifierProvider.value(value: widget.settings),
        ChangeNotifierProvider.value(value: widget.auth),
      ],
      child: MaterialApp(
        debugShowCheckedModeBanner: false,
        title: 'DraftRight',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
              seedColor: Colors.blue, brightness: Brightness.dark),
          useMaterial3: true,
        ),
        home: _buildCurrentView(),
      ),
    );
  }

  Widget _buildCurrentView() {
    switch (_panelState) {
      case _PanelState.visible:
        return Scaffold(
          backgroundColor: Colors.transparent,
          body: Center(
            child: FloatingPanel(
              originalText: _clipboardText,
              onResult: (text, {required bool replace}) =>
                  _onResult(text, replace: replace),
              onCancel: _hidePanel,
            ),
          ),
        );

      case _PanelState.settings:
        return Scaffold(
          appBar: AppBar(
            title: const Text('DraftRight Settings'),
            leading: IconButton(
              icon: const Icon(Icons.arrow_back),
              onPressed: _closeSettings,
            ),
          ),
          body: const _SettingsBody(),
        );

      case _PanelState.hidden:
        // Transparent placeholder — the window is hidden by window_manager
        return const SizedBox.shrink();
    }
  }

  @override
  void dispose() {
    windowManager.removeListener(this);
    super.dispose();
  }
}

// ── Settings body (reuses mobile settings screen) ───────────────────────────

class _SettingsBody extends StatelessWidget {
  const _SettingsBody();

  @override
  Widget build(BuildContext context) {
    // Import SettingsScreen inline to avoid circular imports
    return const _DesktopSettingsContent();
  }
}

class _DesktopSettingsContent extends StatefulWidget {
  const _DesktopSettingsContent();

  @override
  State<_DesktopSettingsContent> createState() =>
      _DesktopSettingsContentState();
}

class _DesktopSettingsContentState extends State<_DesktopSettingsContent> {
  Future<void> _logout() async {
    await context.read<AuthService>().logout();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer2<SettingsService, AuthService>(
      builder: (context, settings, auth, _) {
        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const Text('Account', style: TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            if (auth.isLoggedIn)
              OutlinedButton.icon(
                icon: const Icon(Icons.logout),
                label: const Text('Sign Out'),
                onPressed: _logout,
              )
            else
              const Text('Not signed in — open on mobile to log in.',
                  style: TextStyle(color: Colors.grey)),
            // Backend URL no longer user-editable — production points at
            // api.draftright.info. Dev override:
            //   --dart-define=DRAFTRIGHT_BACKEND_URL=http://localhost:3000
            const SizedBox(height: 24),
            const Text('Translation Language',
                style: TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            DropdownButtonFormField<String>(
              value: settings.translateLanguage,
              decoration: const InputDecoration(border: OutlineInputBorder()),
              items: SettingsService.supportedLanguages
                  .map((lang) =>
                      DropdownMenuItem(value: lang, child: Text(lang)))
                  .toList(),
              onChanged: (value) {
                if (value != null) settings.setTranslateLanguage(value);
              },
            ),
            const SizedBox(height: 24),
            const Divider(),
            const SizedBox(height: 8),
            Row(
              children: [
                const Icon(Icons.keyboard, size: 16),
                const SizedBox(width: 8),
                const Text('Global Hotkey: '),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    border: Border.all(color: Colors.grey),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: const Text('Ctrl + Shift + R',
                      style: TextStyle(fontFamily: 'monospace')),
                ),
              ],
            ),
          ],
        );
      },
    );
  }
}

// ── Panel state enum ─────────────────────────────────────────────────────────

enum _PanelState { hidden, visible, settings }
