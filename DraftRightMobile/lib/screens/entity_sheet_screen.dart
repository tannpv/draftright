import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/entity.dart';
import '../services/extraction_api.dart';

/// Optional smart-scan callback. If null, the Smart-scan button is hidden.
typedef SmartScanFn = Future<List<Entity>> Function(String text);

/// The one-tap action for an entity (call/email/map/open), or null for kinds
/// that are copy-only (OTP, card, bank account, person). Pure + top-level so
/// the kind→URI mapping is unit-testable without launching anything.
({IconData icon, String tooltip, Uri uri})? entityActionFor(Entity e) {
  switch (e.kind) {
    case EntityKind.phone:
      return (icon: Icons.call, tooltip: 'Call', uri: Uri(scheme: 'tel', path: e.value));
    case EntityKind.email:
      return (icon: Icons.send, tooltip: 'Email', uri: Uri(scheme: 'mailto', path: e.value));
    case EntityKind.url:
      final v = e.value.trim();
      return (
        icon: Icons.open_in_new,
        tooltip: 'Open',
        uri: Uri.parse(v.startsWith('http') ? v : 'https://$v'),
      );
    case EntityKind.address:
      return (
        icon: Icons.map,
        tooltip: 'Open in Maps',
        uri: Uri.parse('https://www.google.com/maps/search/?api=1&query=${Uri.encodeComponent(e.value)}'),
      );
    default:
      return null; // otp, creditCard, bankAccount, personName, dateTime → copy only
  }
}

class EntitySheetScreen extends StatefulWidget {
  const EntitySheetScreen({
    super.key,
    required this.text,
    required this.initial,
    required this.smartScan,
  });

  final String text;
  final List<Entity> initial;
  final SmartScanFn? smartScan;

  @override
  State<EntitySheetScreen> createState() => _EntitySheetScreenState();
}

class _EntitySheetScreenState extends State<EntitySheetScreen> {
  late List<Entity> entities;
  bool smartScanLoading = false;
  bool smartScanDone = false;

  @override
  void initState() {
    super.initState();
    // May be empty when the offline pass found nothing — Smart scan is then the
    // fill path (the LLM handles messy layouts the regex layer misses).
    entities = List.of(widget.initial);
  }

  Future<void> _launch(Uri uri) async {
    try {
      final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!ok && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Nothing can open that')),
        );
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not open')),
        );
      }
    }
  }

  Widget _rowActions(EntityKind k, Entity e) {
    final a = entityActionFor(e);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (a != null)
          IconButton(
            key: ValueKey('action-${k.wireName}-${e.value}'),
            icon: Icon(a.icon),
            tooltip: a.tooltip,
            onPressed: () => _launch(a.uri),
          ),
        IconButton(
          key: ValueKey('copy-${k.wireName}-${e.value}'),
          icon: const Icon(Icons.copy),
          tooltip: 'Copy',
          onPressed: () => _copy(e),
        ),
      ],
    );
  }

  Map<EntityKind, List<Entity>> get _grouped {
    final map = <EntityKind, List<Entity>>{};
    for (final e in entities) {
      map.putIfAbsent(e.kind, () => []).add(e);
    }
    return map;
  }

  Future<void> _onSmartScan() async {
    if (widget.smartScan == null || smartScanLoading) return;
    setState(() => smartScanLoading = true);
    try {
      final llm = await widget.smartScan!(widget.text);
      final merged = _merge(entities, llm);
      final added = merged.length - entities.length;
      setState(() {
        entities = merged;
        smartScanDone = true;
        smartScanLoading = false;
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(added > 0
              ? 'Found $added more'
              : 'No additional entities found')),
        );
      }
    } on ExtractionQuotaException {
      setState(() => smartScanLoading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Smart scan limit reached')),
        );
      }
    } catch (_) {
      setState(() => smartScanLoading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Smart scan unavailable — using basic results')),
        );
      }
    }
  }

  List<Entity> _merge(List<Entity> a, List<Entity> b) {
    final seen = {for (final e in a) e.dedupeKey};
    return [...a, ...b.where((e) => seen.add(e.dedupeKey))];
  }

  Future<void> _copy(Entity e) async {
    await Clipboard.setData(ClipboardData(text: e.value));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('${_kindLabel(e.kind)} copied')),
    );
  }

  String _kindLabel(EntityKind k) => switch (k) {
        EntityKind.phone => 'Phone',
        EntityKind.email => 'Email',
        EntityKind.url => 'URL',
        EntityKind.otp => 'OTP',
        EntityKind.creditCard => 'Card',
        EntityKind.address => 'Address',
        EntityKind.personName => 'Person',
        EntityKind.dateTime => 'Date/time',
        EntityKind.bankAccount => 'Bank account',
      };

  IconData _kindIcon(EntityKind k) => switch (k) {
        EntityKind.phone => Icons.phone,
        EntityKind.email => Icons.email,
        EntityKind.url => Icons.link,
        EntityKind.otp => Icons.password,
        EntityKind.creditCard => Icons.credit_card,
        EntityKind.address => Icons.home,
        EntityKind.personName => Icons.person,
        EntityKind.dateTime => Icons.calendar_today,
        EntityKind.bankAccount => Icons.account_balance,
      };

  @override
  Widget build(BuildContext context) {
    final groups = _grouped;
    final orderedKinds = groups.keys.toList()
      ..sort((a, b) => a.index.compareTo(b.index));

    return Scaffold(
      appBar: AppBar(title: const Text('Extracted info')),
      body: ListView(
        padding: const EdgeInsets.all(12),
        children: [
          if (orderedKinds.isEmpty)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 24),
              child: Center(
                child: Text('No info detected yet.',
                    style: TextStyle(color: Colors.grey)),
              ),
            ),
          ...orderedKinds.map((k) => Card(
                margin: const EdgeInsets.only(bottom: 10),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Padding(
                      padding: const EdgeInsets.fromLTRB(14, 12, 14, 4),
                      child: Row(
                        children: [
                          Icon(_kindIcon(k), size: 14, color: Colors.grey),
                          const SizedBox(width: 6),
                          Text(_kindLabel(k),
                              style: const TextStyle(
                                  fontSize: 11,
                                  fontWeight: FontWeight.w600,
                                  color: Colors.grey)),
                        ],
                      ),
                    ),
                    ...groups[k]!.map((e) => ListTile(
                          title: Text(e.display),
                          subtitle: e.source == 'llm'
                              ? const Text('AI',
                                  style: TextStyle(
                                      fontSize: 10, color: Colors.purple))
                              : null,
                          trailing: _rowActions(k, e),
                        )),
                  ],
                ),
              )),
          if (widget.smartScan != null && !smartScanDone)
            FilledButton.icon(
              onPressed: smartScanLoading ? null : _onSmartScan,
              icon: smartScanLoading
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.auto_awesome),
              label: const Text('Smart scan for addresses, names…'),
            ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: () => Navigator.of(context).maybePop(),
                  icon: const Icon(Icons.edit_note, size: 18),
                  label: const Text('Rewrite with tones'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: FilledButton(
                  onPressed: () => Navigator.of(context).maybePop(),
                  child: const Text('Done'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
