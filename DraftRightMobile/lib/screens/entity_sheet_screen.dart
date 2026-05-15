import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../models/entity.dart';
import '../services/extraction_api.dart';

/// Optional smart-scan callback. If null, the Smart-scan button is hidden.
typedef SmartScanFn = Future<List<Entity>> Function(String text);

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
    entities = List.of(widget.initial);
    assert(entities.isNotEmpty,
        'EntitySheetScreen must not be mounted with empty entities');
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
                          trailing: IconButton(
                            key: ValueKey('copy-${k.wireName}-${e.value}'),
                            icon: const Icon(Icons.copy),
                            tooltip: 'Copy',
                            onPressed: () => _copy(e),
                          ),
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
