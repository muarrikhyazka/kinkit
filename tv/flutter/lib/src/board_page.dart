import 'dart:async';

import 'package:flutter/material.dart';

import 'models.dart';
import 'money.dart';
import 'theme.dart';

/// A widget board (the web Widgets page) on the TV: a 12-column flow grid,
/// scaled so the whole board fits the screen. Same layout rules as
/// react/src/WidgetBoard.jsx and tv/web/app.js (renderBoard).
class BoardPage extends StatelessWidget {
  const BoardPage({super.key, required this.board, required this.money});
  final TvBoard board;
  final Money money;

  static const _gap = 14.0;

  @override
  Widget build(BuildContext context) {
    if (board.widgets.isEmpty) {
      return const Center(child: Text('This board is empty.', style: TextStyle(fontSize: 22, color: TvColors.muted)));
    }
    // Greedy lines of at most 12 columns; a line is as tall as its tallest widget.
    final lines = <List<TvWidget>>[[]];
    var used = 0;
    for (final w in board.widgets) {
      if (used + w.w > 12 && lines.last.isNotEmpty) {
        lines.add([]);
        used = 0;
      }
      lines.last.add(w);
      used += w.w;
    }
    int lineH(List<TvWidget> l) => l.fold(1, (m, w) => w.h > m ? w.h : m);
    final units = lines.fold<int>(0, (s, l) => s + lineH(l));
    final gaps = lines.fold<int>(0, (s, l) => s + lineH(l) - 1) + (lines.length - 1);

    return LayoutBuilder(builder: (context, c) {
      final unit = ((c.maxHeight - gaps * _gap) / units).clamp(60.0, 190.0);
      return SingleChildScrollView(
        physics: const NeverScrollableScrollPhysics(),
        child: Column(children: [
          for (var i = 0; i < lines.length; i++) ...[
            if (i > 0) const SizedBox(height: _gap),
            Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              for (var j = 0; j < lines[i].length; j++) ...[
                if (j > 0) const SizedBox(width: _gap),
                SizedBox(
                  width: ((c.maxWidth - _gap * (lines[i].length - 1)) * lines[i][j].w / lines[i].fold<int>(0, (s, w) => s + w.w))
                      .floorToDouble()
                      .clamp(60.0, c.maxWidth),
                  height: lines[i][j].h * unit + (lines[i][j].h - 1) * _gap,
                  child: _Tile(widget: lines[i][j], card: board.cards[lines[i][j].type], money: money),
                ),
              ],
            ]),
          ],
        ]),
      );
    });
  }
}

class _Tile extends StatelessWidget {
  const _Tile({required this.widget, required this.card, required this.money});
  final TvWidget widget;
  final TvCard? card;
  final Money money;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(color: TvColors.card, borderRadius: BorderRadius.circular(14), border: Border.all(color: TvColors.border)),
      child: switch (widget.type) {
        'clock' => const _Clock(),
        'note' => Text(widget.text.isEmpty ? 'Empty note' : widget.text, style: const TextStyle(fontSize: 26, color: TvColors.text)),
        _ => card == null ? const Text('No data', style: TextStyle(color: TvColors.muted)) : _CardBody(card: card!, money: money),
      },
    );
  }
}

class _Clock extends StatefulWidget {
  const _Clock();

  @override
  State<_Clock> createState() => _ClockState();
}

class _ClockState extends State<_Clock> {
  late final Timer _t;
  DateTime _now = DateTime.now();

  @override
  void initState() {
    super.initState();
    _t = Timer.periodic(const Duration(seconds: 1), (_) => setState(() => _now = DateTime.now()));
  }

  @override
  void dispose() {
    _t.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    const wd = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
    final hh = _now.hour.toString().padLeft(2, '0');
    final mm = _now.minute.toString().padLeft(2, '0');
    return Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.start, children: [
      FittedBox(
        fit: BoxFit.scaleDown,
        child: Text('$hh:$mm', style: const TextStyle(fontSize: 68, fontWeight: FontWeight.w800, color: TvColors.text)),
      ),
      Text('${wd[_now.weekday - 1]}, ${_now.day} ${monthLong[_now.month - 1]}', style: const TextStyle(fontSize: 20, color: TvColors.muted)),
    ]);
  }
}

class _CardBody extends StatelessWidget {
  const _CardBody({required this.card, required this.money});
  final TvCard card;
  final Money money;

  @override
  Widget build(BuildContext context) {
    final big = card.bigAmount != null ? money.format(card.bigAmount!) : card.big;
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text(card.title.toUpperCase(), style: const TextStyle(fontSize: 14, letterSpacing: 1.2, fontWeight: FontWeight.w700, color: TvColors.muted)),
      if (big.isNotEmpty)
        FittedBox(
          fit: BoxFit.scaleDown,
          alignment: Alignment.centerLeft,
          child: Text(big,
              style: TextStyle(fontSize: 40, fontWeight: FontWeight.w800, color: (card.bigAmount ?? 0) < 0 ? TvColors.negative : TvColors.text)),
        ),
      if (card.caption.isNotEmpty) Text(card.caption, style: const TextStyle(fontSize: 15, color: TvColors.muted)),
      const SizedBox(height: 4),
      Expanded(
        child: card.rows.isEmpty
            ? (big.isEmpty ? Text(card.empty.isEmpty ? 'Nothing to show' : card.empty, style: const TextStyle(fontSize: 18, color: TvColors.muted)) : const SizedBox.shrink())
            : ClipRect(
                child: SingleChildScrollView(
                  physics: const NeverScrollableScrollPhysics(),
                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    for (final r in card.rows)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Row(children: [
                            Expanded(
                              child: Text(r.title,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 20,
                                    fontWeight: FontWeight.w700,
                                    color: r.alert ? TvColors.negative : TvColors.text,
                                    decoration: r.done ? TextDecoration.lineThrough : null,
                                  )),
                            ),
                            if (r.amount != null) Text(money.format(r.amount!), style: const TextStyle(fontSize: 15, color: TvColors.muted)),
                          ]),
                          if (r.detail.isNotEmpty)
                            Text(r.detail, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 15, color: TvColors.muted)),
                          if (r.pct != null)
                            Padding(
                              padding: const EdgeInsets.only(top: 3),
                              child: ClipRRect(
                                borderRadius: BorderRadius.circular(3),
                                child: LinearProgressIndicator(
                                  minHeight: 6,
                                  value: (r.pct!.clamp(0, 100)) / 100,
                                  color: r.alert ? TvColors.negative : TvColors.primary,
                                  backgroundColor: TvColors.border,
                                ),
                              ),
                            ),
                        ]),
                      ),
                  ]),
                ),
              ),
      ),
    ]);
  }
}
