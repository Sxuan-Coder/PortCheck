//go:build windows

package main

import (
	"reflect"
	"testing"
)

func TestPlanTreeKill(t *testing.T) {
	prot := func(pid uint32) bool { return pid == 0 || pid == 4 || pid == 999 }

	t.Run("链式树按叶子优先排序", func(t *testing.T) {
		entries := []procSnapshotEntry{{1, 0}, {2, 1}, {3, 2}}
		creation := map[uint32]int64{1: 100, 2: 200, 3: 300}
		order, rep := planTreeKill(entries, 1, creation, prot)
		if !reflect.DeepEqual(order, []uint32{3, 2, 1}) {
			t.Fatalf("order = %v, want [3 2 1]", order)
		}
		if rep.Pruned != 0 || rep.Skipped != 0 {
			t.Fatalf("report = %+v", rep)
		}
	})

	t.Run("子进程早于父进程视为复用断链", func(t *testing.T) {
		entries := []procSnapshotEntry{{1, 0}, {2, 1}, {3, 2}}
		creation := map[uint32]int64{1: 100, 2: 50, 3: 300} // 2 早于 1：复用
		order, rep := planTreeKill(entries, 1, creation, prot)
		if !reflect.DeepEqual(order, []uint32{1}) {
			t.Fatalf("order = %v, want [1]", order)
		}
		if rep.Skipped != 0 {
			t.Fatalf("断链不应计入 skipped: %+v", rep)
		}
	})

	t.Run("打不开的进程跳过且不再下钻", func(t *testing.T) {
		entries := []procSnapshotEntry{{1, 0}, {2, 1}, {3, 2}}
		creation := map[uint32]int64{1: 100, 3: 300} // 2 取不到创建时间
		order, rep := planTreeKill(entries, 1, creation, prot)
		if !reflect.DeepEqual(order, []uint32{1}) {
			t.Fatalf("order = %v, want [1]", order)
		}
		if rep.Skipped != 1 {
			t.Fatalf("skipped = %d, want 1", rep.Skipped)
		}
	})

	t.Run("保护命中整棵子树剪枝", func(t *testing.T) {
		entries := []procSnapshotEntry{{1, 0}, {2, 1}, {999, 2}, {888, 999}}
		creation := map[uint32]int64{1: 100, 2: 200, 999: 300, 888: 400}
		order, rep := planTreeKill(entries, 1, creation, prot)
		if !reflect.DeepEqual(order, []uint32{2, 1}) {
			t.Fatalf("order = %v, want [2 1]", order)
		}
		if rep.Pruned != 1 {
			t.Fatalf("pruned = %d, want 1", rep.Pruned)
		}
	})

	t.Run("快照中的环不会死循环", func(t *testing.T) {
		entries := []procSnapshotEntry{{1, 0}, {2, 1}, {1, 2}}
		creation := map[uint32]int64{1: 100, 2: 200}
		order, _ := planTreeKill(entries, 1, creation, prot)
		if !reflect.DeepEqual(order, []uint32{2, 1}) {
			t.Fatalf("order = %v, want [2 1]", order)
		}
	})
}
