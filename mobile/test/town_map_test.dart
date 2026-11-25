import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kinkit_mobile/game/town_map.dart';
import 'package:kinkit_mobile/game/town_map_screen.dart';
import 'package:kinkit_mobile/theme.dart';

void main() {
  final grid = buildGrid();

  test('building ids and labels are unique', () {
    expect({for (final b in kBuildings) b.id}.length, kBuildings.length);
    expect({for (final b in kBuildings) b.label}.length, kBuildings.length);
  });

  test('every door is inside its footprint and walkable', () {
    for (final b in kBuildings) {
      expect(b.contains(b.door.x, b.door.y), isTrue, reason: '${b.id} door outside footprint');
      expect(b.door.y, b.y + b.h - 1, reason: '${b.id} door is not on the bottom row');
      expect(grid.isWalkable(b.door), isTrue, reason: '${b.id} door is blocked');
    }
  });

  test('the tile below every door is walkable and inside the border', () {
    for (final b in kBuildings) {
      expect(grid.isWalkable(b.below), isTrue, reason: '${b.id}: ${b.below} is blocked');
      expect(b.below.y, lessThan(kRows - 1), reason: '${b.id} exit sits on the border');
    }
  });

  test('buildings stay off the border and do not overlap each other or decor', () {
    bool overlap(TileRect a, TileRect b) =>
        a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
    for (final b in kBuildings) {
      expect(b.x, greaterThanOrEqualTo(1), reason: b.id);
      expect(b.y, greaterThanOrEqualTo(1), reason: b.id);
      expect(b.x + b.w, lessThanOrEqualTo(kCols - 1), reason: b.id);
      expect(b.y + b.h, lessThanOrEqualTo(kRows - 1), reason: b.id);
      for (final o in kBuildings) {
        if (!identical(b, o)) expect(overlap(b, o), isFalse, reason: '${b.id} overlaps ${o.id}');
      }
      for (final pond in kPonds) {
        expect(overlap(b, pond), isFalse, reason: '${b.id} overlaps a pond');
      }
      for (final t in kTrees) {
        expect(b.contains(t.x, t.y), isFalse, reason: '${b.id} overlaps a tree');
      }
    }
  });

  test('every exit tile can be reached from the spawn point', () {
    final seen = <Pos>{kSpawn};
    final queue = <Pos>[kSpawn];
    while (queue.isNotEmpty) {
      final p = queue.removeLast();
      for (final d in Dir.values) {
        final n = p.move(d);
        if (grid.isWalkable(n) && seen.add(n)) queue.add(n);
      }
    }
    expect(grid.isWalkable(kSpawn), isTrue);
    for (final b in kBuildings) {
      expect(seen, contains(b.below), reason: '${b.id} exit is cut off');
    }
  });

  test('border, trees, ponds and building walls block; open ground does not', () {
    expect(grid.isSolid(0, 5), isTrue);
    expect(grid.isSolid(kCols - 1, 5), isTrue);
    expect(grid.isSolid(5, 0), isTrue);
    expect(grid.isSolid(5, kRows - 1), isTrue);
    expect(grid.isSolid(-1, 3), isTrue);
    expect(grid.isSolid(3, kRows), isTrue);
    for (final t in kTrees) {
      expect(grid.isSolid(t.x, t.y), isTrue);
    }
    expect(grid.isSolid(1, 11), isTrue); // pond
    expect(grid.isSolid(4, 1), isTrue); // Today wall
    expect(grid.isSolid(kSpawn.x, kSpawn.y), isFalse);
  });

  test('walking into a wall turns Kiki but does not move her', () {
    final left = step(const Pos(1, 4), Dir.left, grid);
    expect(left.pos, const Pos(1, 4));
    expect(left.facing, Dir.left);
    expect(left.enter, isNull);

    // Today's wall, one tile left of its door.
    final wall = step(const Pos(4, 4), Dir.up, grid);
    expect(wall.pos, const Pos(4, 4));
    expect(wall.enter, isNull);
  });

  test('walking on open ground moves one tile and enters nothing', () {
    final r = step(kSpawn, Dir.right, grid);
    expect(r.pos, const Pos(6, 4));
    expect(r.facing, Dir.right);
    expect(r.enter, isNull);
  });

  test('moving up into a door returns the building', () {
    for (final b in kBuildings) {
      final r = step(b.below, Dir.up, grid);
      expect(r.enter, same(b), reason: b.id);
      expect(r.pos, b.door, reason: b.id);
      expect(r.facing, Dir.up);
    }
  });

  test('doorAhead sees the door in front and nothing else', () {
    for (final b in kBuildings) {
      expect(doorAhead(b.below, Dir.up), same(b), reason: b.id);
      expect(doorAhead(b.below, Dir.left), isNull, reason: b.id);
    }
    expect(doorAhead(kSpawn, Dir.up)?.id, 'today');
    expect(doorAhead(kSpawn, Dir.down), isNull);
  });

  test('the spawn point is in front of the Today door', () {
    final today = kBuildings.firstWhere((b) => b.id == 'today');
    expect(kSpawn, today.below);
  });

  group('ground tones', () {
    double contrast(Color a, Color b) {
      final la = a.computeLuminance(), lb = b.computeLuminance();
      final hi = la > lb ? la : lb, lo = la > lb ? lb : la;
      return (hi + 0.05) / (lo + 0.05);
    }

    for (final entry in {'light': KinkitPalette.light, 'dark': KinkitPalette.dark}.entries) {
      test('${entry.key}: roads, trees and the ground are told apart', () {
        final t = TownTones.of(entry.value);
        expect(contrast(t.road, t.ground), greaterThanOrEqualTo(1.5), reason: 'road vs ground');
        expect(contrast(t.tree, t.ground), greaterThanOrEqualTo(1.5), reason: 'tree vs ground');
        expect(contrast(t.roadEdge, t.road), greaterThanOrEqualTo(1.5), reason: 'road edge vs road');
      });
    }
  });
}
