// Pure model of the phone "Town map": a portrait grid of buildings, one per
// main page, that Kiki walks around. No Flutter imports, so it is unit tested
// directly. Web twin (landscape layout): react/src/game/worldMap.js.

/// Map size in tiles. Row 0, row [kRows]-1, column 0 and column [kCols]-1 are a
/// ring of trees.
const kCols = 12;
const kRows = 15;

/// Horizontal road strips (cosmetic: every non-solid tile is walkable). Each
/// one sits just below a row of doors.
const kRoadRows = <int>[4, 7, 10, 13];

enum Dir {
  up(0, -1),
  down(0, 1),
  left(-1, 0),
  right(1, 0);

  final int dx;
  final int dy;
  const Dir(this.dx, this.dy);
}

class Pos {
  final int x;
  final int y;
  const Pos(this.x, this.y);

  Pos move(Dir d) => Pos(x + d.dx, y + d.dy);

  @override
  bool operator ==(Object other) => other is Pos && other.x == x && other.y == y;

  @override
  int get hashCode => Object.hash(x, y);

  @override
  String toString() => 'Pos($x, $y)';
}

/// A rectangle of whole tiles.
class TileRect {
  final int x;
  final int y;
  final int w;
  final int h;
  const TileRect(this.x, this.y, this.w, this.h);

  bool contains(int px, int py) => px >= x && px < x + w && py >= y && py < y + h;
}

/// A building that opens one screen. The door is the bottom-centre tile of the
/// footprint; the tile below it is where Kiki stands after coming back out.
class Building extends TileRect {
  final String id;
  final String label;
  const Building(this.id, this.label, super.x, super.y, super.w, super.h);

  Pos get door => Pos(x + w ~/ 2, y + h - 1);
  Pos get below => Pos(door.x, door.y + 1);
}

/// Buildings ordered like the web nav: Today first, Settings last.
const kBuildings = <Building>[
  Building('today', 'Today', 4, 1, 3, 3),
  Building('calendar', 'Calendar', 1, 1, 2, 2),
  Building('reminders', 'Reminders', 8, 1, 3, 2),
  Building('money', 'Money', 1, 5, 3, 2),
  Building('leaderboard', 'Leaderboard', 5, 5, 3, 2),
  Building('chores', 'Chores', 9, 5, 2, 2),
  Building('lists', 'Lists', 1, 8, 3, 2),
  Building('meals', 'Meals', 5, 8, 2, 2),
  Building('widgets', 'Widgets', 8, 8, 3, 2),
  Building('settings', 'Settings', 5, 11, 2, 2),
];

/// Solid decorations inside the border.
const kTrees = <Pos>[Pos(9, 11), Pos(10, 12)];
const kPonds = <TileRect>[TileRect(1, 11, 2, 2)];

/// Where Kiki starts: on the road in front of the Today door, facing it.
const kSpawn = Pos(5, 4);

/// Which tiles block walking. Everything else, doors included, is walkable.
class TownGrid {
  final int cols;
  final int rows;
  final List<bool> _solid;

  TownGrid._(this.cols, this.rows, this._solid);

  bool isSolid(int x, int y) {
    if (x < 0 || y < 0 || x >= cols || y >= rows) return true;
    return _solid[y * cols + x];
  }

  bool isWalkable(Pos p) => !isSolid(p.x, p.y);
}

/// Solid = border ring, trees, ponds, and every building footprint except its
/// door tile.
TownGrid buildGrid({
  int cols = kCols,
  int rows = kRows,
  List<Building> buildings = kBuildings,
  List<Pos> trees = kTrees,
  List<TileRect> ponds = kPonds,
}) {
  final solid = List<bool>.filled(cols * rows, false);
  void mark(int x, int y) {
    if (x >= 0 && y >= 0 && x < cols && y < rows) solid[y * cols + x] = true;
  }

  for (var x = 0; x < cols; x++) {
    mark(x, 0);
    mark(x, rows - 1);
  }
  for (var y = 0; y < rows; y++) {
    mark(0, y);
    mark(cols - 1, y);
  }
  for (final t in trees) {
    mark(t.x, t.y);
  }
  for (final r in [...ponds, ...buildings]) {
    for (var y = r.y; y < r.y + r.h; y++) {
      for (var x = r.x; x < r.x + r.w; x++) {
        mark(x, y);
      }
    }
  }
  for (final b in buildings) {
    final d = b.door;
    if (d.x >= 0 && d.y >= 0 && d.x < cols && d.y < rows) solid[d.y * cols + d.x] = false;
  }
  return TownGrid._(cols, rows, solid);
}

class StepResult {
  final Pos pos;
  final Dir facing;

  /// The building whose door tile Kiki just stepped onto, if any.
  final Building? enter;
  const StepResult(this.pos, this.facing, this.enter);
}

Building? _doorAt(Pos p, List<Building> buildings) {
  for (final b in buildings) {
    if (b.door == p) return b;
  }
  return null;
}

/// One step. A blocked step turns Kiki but does not move. Stepping onto a door
/// tile moves there and reports the building in [StepResult.enter].
StepResult step(Pos pos, Dir dir, TownGrid grid, [List<Building> buildings = kBuildings]) {
  final next = pos.move(dir);
  if (grid.isSolid(next.x, next.y)) return StepResult(pos, dir, null);
  return StepResult(next, dir, _doorAt(next, buildings));
}

/// The building whose door is the tile directly in front of Kiki, or null.
Building? doorAhead(Pos pos, Dir facing, [List<Building> buildings = kBuildings]) =>
    _doorAt(pos.move(facing), buildings);
