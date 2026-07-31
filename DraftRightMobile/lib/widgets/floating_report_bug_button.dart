import 'dart:ui' as ui;

import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

import 'package:draftright_mobile/widgets/report_bug_sheet.dart';

/// A draggable, edge-snapping floating button that hovers above the app's own
/// screens and opens the bug/feature report sheet on tap.
///
/// Injected once at the app root via `MaterialApp(builder:)` — like
/// [ErrorNoticeOverlay] — so it rides on top of every route with no per-screen
/// wiring. iOS + Android identical.
///
/// Scope note: this floats only above DraftRight's OWN UI (an in-app overlay) —
/// it needs no system permission. (The old system-wide rewrite bubble that drew
/// over *other* apps via SYSTEM_ALERT_WINDOW + an AccessibilityService was
/// removed: banking/security apps block overlays + accessibility, and it gated
/// Play submission.)
///
/// Extend, don't fork: tune via constructor params (enabled, icon, tooltip,
/// size, margin, starting corner) rather than editing the body.
class FloatingReportBugButton extends StatefulWidget {
  /// The app subtree this button floats above.
  final Widget child;

  /// When false the button is not rendered and [child] passes through
  /// untouched — lets a settings toggle gate the feature without unwiring it.
  final bool enabled;

  final IconData icon;
  final String tooltip;

  /// Diameter of the circular button, in logical pixels.
  final double size;

  /// Gap kept between the button and the screen edge (and safe-area insets).
  final double edgeMargin;

  /// Corner the button rests at the first time it appears. Horizontal side is
  /// taken from [Alignment.x] (<=0 → left, else right); vertical fraction from
  /// [Alignment.y] (-1 top … 1 bottom).
  final Alignment startCorner;

  /// Route label recorded in the report's `context` JSON so triagers can see
  /// the entry point.
  final String routeLabel;

  const FloatingReportBugButton({
    super.key,
    required this.child,
    this.enabled = true,
    this.icon = Icons.bug_report_outlined,
    this.tooltip = 'Report a bug',
    this.size = 52,
    this.edgeMargin = 12,
    this.startCorner = Alignment.centerRight,
    this.routeLabel = '/floating-report',
  });

  @override
  State<FloatingReportBugButton> createState() =>
      _FloatingReportBugButtonState();
}

class _FloatingReportBugButtonState extends State<FloatingReportBugButton> {
  // Top-left offset of the button in logical px. Null until the first layout
  // reveals the viewport size, then dragged around and snapped to an edge.
  Offset? _pos;
  bool _dragging = false;

  static const Duration _snapDuration = Duration(milliseconds: 200);

  /// The button's starting resting spot within the padded, safe area.
  Offset _cornerPosition(Size area, EdgeInsets pad) {
    final minX = widget.edgeMargin + pad.left;
    final maxX = area.width - widget.size - widget.edgeMargin - pad.right;
    final minY = widget.edgeMargin + pad.top;
    final maxY = area.height - widget.size - widget.edgeMargin - pad.bottom;
    final x = widget.startCorner.x <= 0 ? minX : maxX;
    final yFraction = (widget.startCorner.y + 1) / 2; // -1..1 → 0..1
    final y = minY + yFraction * (maxY - minY);
    return Offset(x, y);
  }

  /// Keep the button fully on-screen inside the safe area — used both while
  /// dragging and on every rebuild (the viewport shrinks when the keyboard
  /// opens, which could otherwise strand the button off-screen).
  Offset _clamp(Offset p, Size area, EdgeInsets pad) {
    final minX = widget.edgeMargin + pad.left;
    final maxX = area.width - widget.size - widget.edgeMargin - pad.right;
    final minY = widget.edgeMargin + pad.top;
    final maxY = area.height - widget.size - widget.edgeMargin - pad.bottom;
    return Offset(
      p.dx.clamp(minX, maxX < minX ? minX : maxX),
      p.dy.clamp(minY, maxY < minY ? minY : maxY),
    );
  }

  /// Snap to whichever vertical edge (left/right) is nearer, keeping height.
  Offset _snapToEdge(Offset p, Size area, EdgeInsets pad) {
    final minX = widget.edgeMargin + pad.left;
    final maxX = area.width - widget.size - widget.edgeMargin - pad.right;
    final center = p.dx + widget.size / 2;
    final x = center <= area.width / 2 ? minX : maxX;
    return Offset(x, p.dy);
  }

  // Wraps the app content (NOT the button) so a capture excludes the FAB.
  final GlobalKey _captureKey = GlobalKey();

  /// PNG of the screen the user is on, captured from the app's own render tree
  /// (in-app, no permission). Best-effort — returns null on any failure. (#135)
  Future<Uint8List?> _captureScreen() async {
    try {
      final boundary =
          _captureKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
      if (boundary == null) return null;
      final dpr = MediaQuery.of(context).devicePixelRatio.clamp(1.0, 2.0);
      final image = await boundary.toImage(pixelRatio: dpr);
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      image.dispose();
      return data?.buffer.asUint8List();
    } catch (_) {
      return null;
    }
  }

  Future<void> _openReport() async {
    // Capture BEFORE the sheet is shown, so the shot is the buggy screen.
    final shot = await _captureScreen();
    if (!mounted) return;
    await showReportBugSheet(
      context,
      currentRoute: widget.routeLabel,
      initialScreenshotBytes: shot,
    );
  }

  @override
  Widget build(BuildContext context) {
    if (!widget.enabled) return widget.child;

    final pad = MediaQuery.of(context).padding;
    return LayoutBuilder(
      builder: (context, constraints) {
        final area = Size(constraints.maxWidth, constraints.maxHeight);
        // Establish (or re-clamp) the resting position for this viewport.
        final pos = _clamp(_pos ?? _cornerPosition(area, pad), area, pad);
        return Stack(
          children: [
            // Capture target: the app content only (the FAB sits above this
            // boundary, so it's never in the screenshot). #135
            RepaintBoundary(key: _captureKey, child: widget.child),
            AnimatedPositioned(
              // Instant while the finger drives it; eased on the edge-snap.
              duration: _dragging ? Duration.zero : _snapDuration,
              curve: Curves.easeOut,
              left: pos.dx,
              top: pos.dy,
              child: GestureDetector(
                onPanStart: (_) => setState(() {
                  _dragging = true;
                  _pos = pos;
                }),
                onPanUpdate: (d) => setState(() {
                  _pos = _clamp((_pos ?? pos) + d.delta, area, pad);
                }),
                onPanEnd: (_) => setState(() {
                  _dragging = false;
                  _pos = _snapToEdge(_pos ?? pos, area, pad);
                }),
                onTap: _openReport,
                child: _bubble(context),
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _bubble(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Tooltip(
      message: widget.tooltip,
      child: Material(
        elevation: _dragging ? 10 : 4,
        color: scheme.primary,
        shape: const CircleBorder(),
        child: SizedBox(
          width: widget.size,
          height: widget.size,
          child: Icon(
            widget.icon,
            color: scheme.onPrimary,
            size: widget.size * 0.5,
          ),
        ),
      ),
    );
  }
}
