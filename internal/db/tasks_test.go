package db

import (
	"testing"

	"github.com/vladimirkreslin/todoshka/internal/models"
)

func TestTaskCreateAndList(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, err := d.CreateTask(uid, &models.Task{Title: "Buy milk", Status: "todo", Priority: "high"})
	if err != nil { t.Fatal(err) }
	list, _ := d.ListTasksForUser(uid, TaskFilter{})
	if len(list) != 1 || list[0].Title != "Buy milk" || list[0].ID != id {
		t.Fatalf("got %+v", list)
	}
}

func TestTaskUpdateAndDelete(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	id, _ := d.CreateTask(uid, &models.Task{Title: "A"})
	desc := "new"
	if err := d.UpdateTask(id, uid, map[string]any{"description": desc, "status": "done"}); err != nil { t.Fatal(err) }
	got, _ := d.GetTask(id, uid)
	if got.Status != "done" || got.Description == nil || *got.Description != desc { t.Fatalf("%+v", got) }
	if err := d.DeleteTask(id, uid); err != nil { t.Fatal(err) }
	if _, err := d.GetTask(id, uid); err == nil { t.Fatal("expected not found") }
}

func TestTaskAccessControl(t *testing.T) {
	d := newTestDB(t)
	owner, _ := d.CreateUser("alice", "x")
	other, _ := d.CreateUser("bob", "x")
	id, _ := d.CreateTask(owner, &models.Task{Title: "A"})
	if _, err := d.GetTask(id, other); err == nil { t.Fatal("bob should not see alice's task") }
	if err := d.Share("task", id, other); err != nil { t.Fatal(err) }
	if _, err := d.GetTask(id, other); err != nil { t.Fatal("bob should see after share") }
}

func TestSubtasksAndTags(t *testing.T) {
	d := newTestDB(t)
	uid, _ := d.CreateUser("alice", "x")
	tid, _ := d.CreateTask(uid, &models.Task{Title: "A", Tags: []string{"work", "urgent"}})
	sid, err := d.CreateSubtask(tid, uid, "step 1")
	if err != nil { t.Fatal(err) }
	if err := d.UpdateSubtask(sid, uid, map[string]any{"done": true}); err != nil { t.Fatal(err) }
	task, _ := d.GetTask(tid, uid)
	if !task.Subtasks[0].Done { t.Fatal("not done") }
	if len(task.Tags) != 2 { t.Fatalf("tags: %+v", task.Tags) }
	if err := d.AddTaskTag(tid, uid, "home"); err != nil { t.Fatal(err) }
	if err := d.RemoveTaskTag(tid, uid, "work"); err != nil { t.Fatal(err) }
	if err := d.DeleteSubtask(sid, uid); err != nil { t.Fatal(err) }
}
