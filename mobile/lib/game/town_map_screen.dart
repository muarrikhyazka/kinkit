import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../screens/calendar_page.dart';
import '../screens/chores_screen.dart';
import '../screens/family_screen.dart';
import '../screens/lists_screen.dart';
import '../screens/meals_screen.dart';
import '../screens/money_screen.dart';
import '../screens/reminders_screen.dart';
import '../screens/settings_screen.dart';
import '../screens/today_screen.dart';
import '../screens/widgets_screen.dart';
import '../theme.dart';
import '../widgets/k_app_bar.dart';
import '../widgets/mascot.dart';
import 'town_map.dart';

/// The page a building opens. Same screens More and the bottom bar use.
Widget _screenFor(String id) {
  switch (id) {
    case 'today':
      return const TodayScreen();
    case 'calendar':
      return const CalendarScreen();
    case 'reminders':
      return const RemindersScreen();
    case 'money':
      return const MoneyScreen();
    case 'leaderboard':
      return const FamilyScreen();
    case 'chores':
      return const ChoresScreen();
    case 'lists':
      return const ListsScreen();
    case 'meals':
      return const MealsScreen();
    case 'widgets':
      return const WidgetsScreen();
    case 'settings':
      return const SettingsScreen();
  }
  throw ArgumentError.value(id, 'id', 'No screen for this building');
}

const _kPadButton = 52.0;
const _kStepAnimation = Duration(milliseconds: 120);
const _kMinStepGap = Duration(milliseconds: 100);
const _kHoldRepeat = Duration(milliseconds: 160);

/// Game-mode navigation: walk Kiki around a small town; each building opens
/// one page. Replaces the Today tab when Settings > Navigation is "Town map".
/// Arrow keys or WASD move, Enter opens the door ahead, and every building can
/// also just be tapped.
class TownMapScreen extends ConsumerStatefulWidget {
  const TownMapScreen({super.key});

  @override
  ConsumerState<TownMapScreen> createState() => _TownMapScreenState();
}

class _TownMapScreenState extends ConsumerState<TownMapScreen> {
  static final _keyDirs = <LogicalKeyboardKey, Dir>{
    LogicalKeyboardKey.arrowUp: Dir.up,
    LogicalKeyboardKey.arrowDown: Dir.down,
    LogicalKeyboardKey.arrowLeft: Dir.left,
    LogicalKeyboardKey.arrowRight: Dir.right,
    LogicalKeyboardKey.keyW: Dir.up,
    LogicalKeyboardKey.keyS: Dir.down,
    LogicalKeyboardKey.keyA: Dir.left,
    LogicalKeyboardKey.keyD: Dir.right,
  };

  final FocusNode _focus = FocusNode(debugLabel: 'Town map');
  final TownGrid _grid = buildGrid();

  Pos _pos = kSpawn;
  Dir _facing = Dir.up;
  bool _opening = false;
  String? _focusedBuilding;
  Timer? _holdTimer;
  DateTime _lastKeyStep = DateTime.fromMillisecondsSinceEpoch(0);

  @override
  void dispose() {
    _holdTimer?.cancel();
    _focus.dispose();
    super.dispose();
  }

  void _move(Dir d) {
    if (_opening) return;
    final r = step(_pos, d, _grid);
    setState(() {
      _pos = r.pos;
      _facing = r.facing;
    });
    final b = r.enter;
    if (b != null) _open(b);
  }

  void _startHold(Dir d) {
    _stopHold();
    _move(d);
    _holdTimer = Timer.periodic(_kHoldRepeat, (_) => _move(d));
  }

  void _stopHold() {
    _holdTimer?.cancel();
    _holdTimer = null;
  }

  Future<void> _open(Building b) async {
    if (_opening) return;
    _stopHold();
    setState(() => _opening = true);
    await Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => _screenFor(b.id)));
    if (!mounted) return;
    // Back outside: stand in front of the door just used, facing it.
    setState(() {
      _opening = false;
      _pos = b.below;
      _facing = Dir.up;
    });
    _focus.requestFocus();
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is KeyUpEvent) return KeyEventResult.ignored;
    // Offstage tabs keep their state but must not walk.
    if (!TickerMode.of(context)) return KeyEventResult.ignored;
    final keyboard = HardwareKeyboard.instance;
    if (keyboard.isControlPressed || keyboard.isMetaPressed || keyboard.isAltPressed) {
      return KeyEventResult.ignored;
    }
    final key = event.logicalKey;
    final dir = _keyDirs[key];
    if (dir != null) {
      final now = DateTime.now();
      if (now.difference(_lastKeyStep) >= _kMinStepGap) {
        _lastKeyStep = now;
        _move(dir);
      }
      return KeyEventResult.handled;
    }
    // Enter and Space belong to a focused button; only act when the map itself has focus.
    if (event is KeyDownEvent &&
        node.hasPrimaryFocus &&
        (key == LogicalKeyboardKey.enter || key == LogicalKeyboardKey.numpadEnter || key == LogicalKeyboardKey.space)) {
      final ahead = doorAhead(_pos, _facing);
      if (ahead != null) _open(ahead);
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  void _onBuildingFocus(String id, bool focused) {
    setState(() {
      if (focused) {
        _focusedBuilding = id;
      } else if (_focusedBuilding == id) {
        _focusedBuilding = null;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    final ahead = doorAhead(_pos, _facing);
    final status = ahead == null
        ? 'Use the pad or arrow keys to walk. Walk into a door to open a page.'
        : '${ahead.label} is ahead. Walk in or press Open.';

    final hint = Semantics(
      liveRegion: true,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minHeight: 48),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: KSpace.s4, vertical: KSpace.s2),
          child: Align(
            alignment: Alignment.centerLeft,
            child: Text(
              status,
              style: TextStyle(
                fontSize: 14,
                fontWeight: ahead == null ? FontWeight.w500 : FontWeight.w600,
                color: ahead == null ? p.muted : p.text,
              ),
            ),
          ),
        ),
      ),
    );

    final pad = _DPad(
      ahead: ahead,
      onMove: _move,
      onHoldStart: _startHold,
      onHoldEnd: _stopHold,
      onOpen: ahead == null ? null : () => _open(ahead),
    );

    return Scaffold(
      appBar: kAppBar(context, 'Town map'),
      body: Focus(
        focusNode: _focus,
        autofocus: true,
        onKeyEvent: _onKey,
        child: SafeArea(
          top: false,
          bottom: false,
          child: LayoutBuilder(
            builder: (context, c) {
              if (c.maxWidth > c.maxHeight) {
                // Landscape: pad on the left so the quick-add button stays clear.
                return Row(
                  children: [
                    SizedBox(
                      width: 220,
                      child: Center(child: SingleChildScrollView(child: pad)),
                    ),
                    Expanded(
                      child: Column(children: [hint, Expanded(child: _buildMap(p))]),
                    ),
                  ],
                );
              }
              return Column(children: [hint, Expanded(child: _buildMap(p)), pad]);
            },
          ),
        ),
      ),
    );
  }

  Widget _buildMap(KinkitPalette p) {
    final reduce = MediaQuery.maybeDisableAnimationsOf(context) ?? false;
    final duration = reduce ? Duration.zero : _kStepAnimation;
    return LayoutBuilder(
      builder: (context, c) {
        final tile = (c.maxWidth / kCols).clamp(24.0, 56.0).toDouble();
        final worldW = tile * kCols;
        final worldH = tile * kRows;
        final ox = _cameraOffset(c.maxWidth, worldW, (_pos.x + 0.5) * tile);
        final oy = _cameraOffset(c.maxHeight, worldH, (_pos.y + 0.5) * tile);
        return ClipRect(
          child: Stack(
            fit: StackFit.expand,
            children: [
              AnimatedPositioned(
                duration: duration,
                curve: Curves.linear,
                left: ox,
                top: oy,
                width: worldW,
                height: worldH,
                child: Stack(
                  clipBehavior: Clip.none,
                  children: [
                    Positioned.fill(
                      child: CustomPaint(painter: _TownPainter(tile: tile, palette: p)),
                    ),
                    for (final b in kBuildings) _buildingWidget(b, tile, p),
                    AnimatedPositioned(
                      duration: duration,
                      curve: Curves.linear,
                      left: _pos.x * tile,
                      top: _pos.y * tile,
                      width: tile,
                      height: tile,
                      child: ExcludeSemantics(
                        child: IgnorePointer(
                          child: Transform.flip(
                            flipX: _facing == Dir.left,
                            child: Mascot(size: tile, animate: false),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  /// Keeps Kiki centred, but never shows past the edge of the world; a world
  /// smaller than the viewport is centred instead.
  static double _cameraOffset(double viewport, double world, double target) {
    if (world <= viewport) return (viewport - world) / 2;
    return (viewport / 2 - target).clamp(viewport - world, 0.0).toDouble();
  }

  Widget _buildingWidget(Building b, double tile, KinkitPalette p) {
    final w = b.w * tile;
    final h = b.h * tile;
    final roofH = tile * (b.h >= 3 ? 1.2 : 0.8);
    final focused = _focusedBuilding == b.id;
    return Positioned(
      left: b.x * tile,
      top: b.y * tile,
      width: w,
      height: h,
      child: Semantics(
        button: true,
        label: 'Open ${b.label}',
        excludeSemantics: true,
        onTap: () => _open(b),
        child: Stack(
          children: [
            Positioned(
              left: 0,
              right: 0,
              top: 0,
              height: roofH,
              child: CustomPaint(painter: _RoofPainter(color: p.primary)),
            ),
            Positioned(
              left: 0,
              right: 0,
              top: roofH,
              bottom: 0,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: p.card,
                  border: Border.all(color: focused ? p.primary : p.borderStrong, width: focused ? 3 : 1.5),
                  borderRadius: const BorderRadius.vertical(bottom: Radius.circular(KRadius.sm)),
                ),
                child: Stack(
                  children: [
                    Positioned.fill(
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(3, 2, 3, 0),
                        child: Align(
                          alignment: Alignment.topCenter,
                          child: FittedBox(
                            fit: BoxFit.scaleDown,
                            child: Text(
                              b.label,
                              maxLines: 1,
                              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: p.text),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Align(
                      alignment: Alignment.bottomCenter,
                      child: Container(
                        width: tile * 0.5,
                        height: tile * 0.45,
                        decoration: BoxDecoration(
                          color: p.primary,
                          borderRadius: const BorderRadius.vertical(top: Radius.circular(KRadius.sm)),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            Positioned.fill(
              child: Material(
                type: MaterialType.transparency,
                child: InkWell(
                  borderRadius: BorderRadius.circular(KRadius.sm),
                  onFocusChange: (f) => _onBuildingFocus(b.id, f),
                  onTap: () => _open(b),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Map colours, all derived from existing palette tokens: a lavender ground
/// (primaryLight pulled toward primary), roads in the page background, trees in
/// the success green. Roads and trees keep at least 1.5:1 luminance contrast
/// against the ground in both themes (checked in town_map_test.dart).
@visibleForTesting
class TownTones {
  final Color ground;
  final Color road;
  final Color roadEdge;
  final Color tree;
  final Color treeRing;

  const TownTones._(this.ground, this.road, this.roadEdge, this.tree, this.treeRing);

  factory TownTones.of(KinkitPalette p) => TownTones._(
        Color.lerp(p.primaryLight, p.primary, 0.3)!,
        p.bg,
        p.muted,
        p.success,
        p.card,
      );
}

/// Ground, roads, the tree border, trees and ponds. Everything that is not a
/// building or Kiki, drawn in one pass from the theme palette.
class _TownPainter extends CustomPainter {
  final double tile;
  final KinkitPalette palette;
  const _TownPainter({required this.tile, required this.palette});

  @override
  void paint(Canvas canvas, Size size) {
    final p = palette;
    final tones = TownTones.of(p);
    canvas.drawRect(Offset.zero & size, Paint()..color = tones.ground);

    final roadFill = Paint()..color = tones.road;
    final roadEdge = Paint()
      ..color = tones.roadEdge
      ..strokeWidth = 1;
    for (final row in kRoadRows) {
      final left = tile;
      final right = (kCols - 1) * tile;
      canvas.drawRect(Rect.fromLTRB(left, row * tile, right, (row + 1) * tile), roadFill);
      canvas.drawLine(Offset(left, row * tile), Offset(right, row * tile), roadEdge);
      canvas.drawLine(Offset(left, (row + 1) * tile), Offset(right, (row + 1) * tile), roadEdge);
    }

    for (final pond in kPonds) {
      final r = RRect.fromRectAndRadius(
        Rect.fromLTWH(pond.x * tile, pond.y * tile, pond.w * tile, pond.h * tile).deflate(tile * 0.06),
        Radius.circular(tile * 0.4),
      );
      canvas.drawRRect(r, Paint()..color = p.primaryLight);
      canvas.drawRRect(
        r,
        Paint()
          ..color = p.primary
          ..style = PaintingStyle.stroke
          ..strokeWidth = 1.5,
      );
    }

    final treeFill = Paint()..color = tones.tree;
    final treeRing = Paint()
      ..color = tones.treeRing
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.2;
    void tree(int x, int y) {
      final c = Offset((x + 0.5) * tile, (y + 0.5) * tile);
      canvas.drawCircle(c, tile * 0.4, treeFill);
      canvas.drawCircle(c, tile * 0.4, treeRing);
    }

    for (var x = 0; x < kCols; x++) {
      tree(x, 0);
      tree(x, kRows - 1);
    }
    for (var y = 1; y < kRows - 1; y++) {
      tree(0, y);
      tree(kCols - 1, y);
    }
    for (final t in kTrees) {
      tree(t.x, t.y);
    }
  }

  @override
  bool shouldRepaint(_TownPainter old) => old.tile != tile || old.palette != palette;
}

/// The house roof from the logo: a flat triangle.
class _RoofPainter extends CustomPainter {
  final Color color;
  const _RoofPainter({required this.color});

  @override
  void paint(Canvas canvas, Size size) {
    final path = Path()
      ..moveTo(size.width / 2, 0)
      ..lineTo(size.width, size.height)
      ..lineTo(0, size.height)
      ..close();
    canvas.drawPath(path, Paint()..color = color);
  }

  @override
  bool shouldRepaint(_RoofPainter old) => old.color != color;
}

/// Four direction buttons plus "Open" in the middle. A tap steps once; holding
/// repeats. Open is enabled only when a door is directly ahead.
class _DPad extends StatelessWidget {
  final Building? ahead;
  final ValueChanged<Dir> onMove;
  final ValueChanged<Dir> onHoldStart;
  final VoidCallback onHoldEnd;
  final VoidCallback? onOpen;

  const _DPad({
    required this.ahead,
    required this.onMove,
    required this.onHoldStart,
    required this.onHoldEnd,
    required this.onOpen,
  });

  @override
  Widget build(BuildContext context) {
    final p = context.palette;
    const gap = SizedBox(width: KSpace.s2, height: KSpace.s2);
    const blank = SizedBox.square(dimension: _kPadButton);

    Widget move(Dir d, IconData icon, String label) => _PadButton(
          icon: icon,
          label: label,
          background: p.primaryLight,
          foreground: p.text,
          border: p.primary,
          onTap: () => onMove(d),
          onHoldStart: () => onHoldStart(d),
          onHoldEnd: onHoldEnd,
        );

    final openLabel = ahead == null ? 'Open door ahead' : 'Open door ahead: ${ahead!.label}';

    return DecoratedBox(
      decoration: BoxDecoration(
        color: p.card,
        border: Border(top: BorderSide(color: p.border)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(KSpace.s3),
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [blank, gap, move(Dir.up, Icons.keyboard_arrow_up, 'Move up'), gap, blank],
              ),
              gap,
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  move(Dir.left, Icons.keyboard_arrow_left, 'Move left'),
                  gap,
                  _PadButton(
                    icon: Icons.login,
                    label: openLabel,
                    background: p.primaryFill,
                    foreground: p.onPrimary,
                    border: p.primaryFill,
                    disabledBackground: p.surface2,
                    disabledForeground: p.muted,
                    onTap: onOpen,
                  ),
                  gap,
                  move(Dir.right, Icons.keyboard_arrow_right, 'Move right'),
                ],
              ),
              gap,
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [blank, gap, move(Dir.down, Icons.keyboard_arrow_down, 'Move down'), gap, blank],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _PadButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color background;
  final Color foreground;
  final Color border;
  final Color? disabledBackground;
  final Color? disabledForeground;
  final VoidCallback? onTap;
  final VoidCallback? onHoldStart;
  final VoidCallback? onHoldEnd;

  const _PadButton({
    required this.icon,
    required this.label,
    required this.background,
    required this.foreground,
    required this.border,
    required this.onTap,
    this.disabledBackground,
    this.disabledForeground,
    this.onHoldStart,
    this.onHoldEnd,
  });

  @override
  Widget build(BuildContext context) {
    final button = IconButton(
      onPressed: onTap,
      icon: Icon(icon, size: 28),
      style: IconButton.styleFrom(
        fixedSize: const Size.square(_kPadButton),
        backgroundColor: background,
        foregroundColor: foreground,
        disabledBackgroundColor: disabledBackground,
        disabledForegroundColor: disabledForeground,
        side: BorderSide(color: onTap == null ? Colors.transparent : border, width: 1.5),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(KRadius.md)),
      ),
    );
    // Long press belongs to the repeat, so the tooltip is hover-only (manual
    // trigger); the Semantics node carries the spoken label.
    return Semantics(
      button: true,
      enabled: onTap != null,
      label: label,
      onTap: onTap,
      excludeSemantics: true,
      child: Tooltip(
        message: label,
        triggerMode: TooltipTriggerMode.manual,
        excludeFromSemantics: true,
        child: onHoldStart == null
            ? button
            : GestureDetector(
                onLongPressStart: (_) => onHoldStart!(),
                onLongPressEnd: (_) => onHoldEnd?.call(),
                onLongPressCancel: () => onHoldEnd?.call(),
                child: button,
              ),
      ),
    );
  }
}
