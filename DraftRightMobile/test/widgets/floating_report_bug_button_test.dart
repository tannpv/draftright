import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:draftright_mobile/widgets/floating_report_bug_button.dart';

void main() {
  // A tappable target inside [child] lets us prove the button overlays the app
  // without swallowing the app's own hit-testing outside its footprint.
  Widget host({bool enabled = true}) => MaterialApp(
        home: FloatingReportBugButton(
          enabled: enabled,
          child: Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () {},
                child: const Text('app content'),
              ),
            ),
          ),
        ),
      );

  testWidgets('renders the report button above the app child', (tester) async {
    await tester.pumpWidget(host());
    expect(find.byIcon(Icons.bug_report_outlined), findsOneWidget);
    expect(find.text('app content'), findsOneWidget);
  });

  testWidgets('enabled:false passes the child through with no button',
      (tester) async {
    await tester.pumpWidget(host(enabled: false));
    expect(find.byIcon(Icons.bug_report_outlined), findsNothing);
    expect(find.text('app content'), findsOneWidget);
  });

  testWidgets('snaps to the nearer edge after a drag', (tester) async {
    await tester.pumpWidget(host());
    await tester.pumpAndSettle();

    final icon = find.byIcon(Icons.bug_report_outlined);
    final startCenter = tester.getCenter(icon);

    // Default startCorner is centerRight → button sits on the right edge.
    // Drag it well past screen-centre toward the left, release, let it snap.
    await tester.drag(icon, const Offset(-500, 0));
    await tester.pumpAndSettle();

    final endCenter = tester.getCenter(icon);
    expect(endCenter.dx, lessThan(startCenter.dx),
        reason: 'dragging left of centre should snap the button to the left edge');
  });
}
