// Package taskjson is the single canonical JSON shape for an Asana task.
//
// `tasks view`, `tasks list`, `tasks search` and `tasks create` each used to
// hand-roll their own anonymous structs for this, and they had drifted: search
// marked `completed` omitempty where the others did not, and list and search
// omitted fields view emitted. One shape means one parser handles the output of
// all four, which is what a caller that creates a task and later re-fetches it
// actually needs.
package taskjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/jtsternberg/asana-cli/internal/api/asana"
)

// Ref is a compact reference to another object.
type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// CustomField is one custom field value on a task. display_value is emitted even
// when null, so a consumer can tell "no value" from "not returned".
type CustomField struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	DisplayValue *string `json:"display_value"`
}

// Membership is a task's placement in a project section.
type Membership struct {
	Project *Ref `json:"project,omitempty"`
	Section *Ref `json:"section,omitempty"`
}

// Task is the canonical serialized form of an asana.Task.
//
// The identifier key is "id" rather than the API's "gid". Field order here is
// the key order in the output.
//
// assignee, completed, num_subtasks, liked and num_likes are deliberately not
// omitempty: "unassigned" and "incomplete" are facts worth recording, and a
// consumer should not have to guess whether a missing key means false or absent.
type Task struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	ResourceSubtype string         `json:"resource_subtype,omitempty"`
	Assignee        *Ref           `json:"assignee"`
	Completed       *bool          `json:"completed"`
	CompletedAt     string         `json:"completed_at,omitempty"`
	CreatedAt       string         `json:"created_at,omitempty"`
	ModifiedAt      string         `json:"modified_at,omitempty"`
	DueOn           string         `json:"due_on,omitempty"`
	DueAt           string         `json:"due_at,omitempty"`
	StartOn         string         `json:"start_on,omitempty"`
	Notes           string         `json:"notes,omitempty"`
	HTMLNotes       string         `json:"html_notes,omitempty"`
	Parent          *Ref           `json:"parent,omitempty"`
	Projects        []*Ref         `json:"projects,omitempty"`
	Tags            []*Ref         `json:"tags,omitempty"`
	Memberships     []*Membership  `json:"memberships,omitempty"`
	CustomFields    []*CustomField `json:"custom_fields,omitempty"`
	Dependencies    []*Ref         `json:"dependencies,omitempty"`
	Dependents      []*Ref         `json:"dependents,omitempty"`
	Followers       []*Ref         `json:"followers,omitempty"`
	Workspace       *Ref           `json:"workspace,omitempty"`
	NumSubtasks     int32          `json:"num_subtasks"`
	Liked           bool           `json:"liked"`
	NumLikes        int32          `json:"num_likes"`
	PermalinkURL    string         `json:"permalink_url,omitempty"`
}

// fields is the opt_fields list that populates every key in Task.
//
// opt_fields replaces Asana's default field set rather than adding to it, so
// this list has to be complete — html_notes, num_subtasks, dependencies and
// dependents are not returned by default at all. TestFields_CoversEveryKeyInTheShape
// keeps it in step with the struct above.
var fields = []string{
	"name",
	"resource_subtype",
	"assignee",
	"assignee.name",
	"completed",
	"completed_at",
	"created_at",
	"modified_at",
	"due_on",
	"due_at",
	"start_on",
	"notes",
	"html_notes",
	"parent",
	"parent.name",
	"projects",
	"projects.name",
	"tags",
	"tags.name",
	"memberships",
	"memberships.project",
	"memberships.project.name",
	"memberships.section",
	"memberships.section.name",
	"custom_fields",
	"custom_fields.name",
	"custom_fields.display_value",
	"dependencies",
	"dependencies.name",
	"dependents",
	"dependents.name",
	"followers",
	"followers.name",
	"workspace",
	"workspace.name",
	"num_subtasks",
	"liked",
	"num_likes",
	"permalink_url",
}

// Fields returns the canonical opt_fields list. The returned slice is a copy.
func Fields() []string {
	out := make([]string, len(fields))
	copy(out, fields)
	return out
}

// Options returns request options that ask Asana for every field the canonical
// shape can carry.
func Options() *asana.Options {
	return &asana.Options{Fields: Fields()}
}

// New converts an asana.Task into the canonical shape.
func New(task *asana.Task) *Task {
	if task == nil {
		return nil
	}

	out := &Task{
		ID:              task.ID,
		Name:            task.Name,
		ResourceSubtype: task.ResourceSubtype,
		Completed:       task.Completed,
		Notes:           task.Notes,
		HTMLNotes:       task.HTMLNotes,
		NumSubtasks:     task.NumSubtasks,
		Liked:           task.Liked,
		NumLikes:        task.NumLikes,
		PermalinkURL:    task.PermalinkURL,
	}

	if task.Assignee != nil {
		out.Assignee = &Ref{ID: task.Assignee.ID, Name: task.Assignee.Name}
	}
	if task.Parent != nil {
		out.Parent = &Ref{ID: task.Parent.ID, Name: task.Parent.Name}
	}
	if task.Workspace != nil {
		out.Workspace = &Ref{ID: task.Workspace.ID, Name: task.Workspace.Name}
	}
	if task.DueOn != nil {
		out.DueOn = time.Time(*task.DueOn).Format(time.DateOnly)
	}
	if task.DueAt != nil {
		out.DueAt = task.DueAt.Format(time.RFC3339)
	}
	if task.StartOn != nil {
		out.StartOn = time.Time(*task.StartOn).Format(time.DateOnly)
	}
	if task.CreatedAt != nil {
		out.CreatedAt = task.CreatedAt.Format(time.RFC3339)
	}
	if task.ModifiedAt != nil {
		out.ModifiedAt = task.ModifiedAt.Format(time.RFC3339)
	}
	if task.CompletedAt != nil {
		out.CompletedAt = task.CompletedAt.Format(time.RFC3339)
	}
	for _, p := range task.Projects {
		out.Projects = append(out.Projects, &Ref{ID: p.ID, Name: p.Name})
	}
	for _, tag := range task.Tags {
		out.Tags = append(out.Tags, &Ref{ID: tag.ID, Name: tag.Name})
	}
	for _, m := range task.Memberships {
		membership := &Membership{}
		if m.Project != nil {
			membership.Project = &Ref{ID: m.Project.ID, Name: m.Project.Name}
		}
		if m.Section != nil {
			membership.Section = &Ref{ID: m.Section.ID, Name: m.Section.Name}
		}
		out.Memberships = append(out.Memberships, membership)
	}
	for _, cf := range task.CustomFields {
		out.CustomFields = append(out.CustomFields, &CustomField{
			ID:           cf.ID,
			Name:         cf.Name,
			DisplayValue: cf.DisplayValue,
		})
	}
	for _, d := range task.Dependencies {
		out.Dependencies = append(out.Dependencies, &Ref{ID: d.ID, Name: d.Name})
	}
	for _, d := range task.Dependents {
		out.Dependents = append(out.Dependents, &Ref{ID: d.ID, Name: d.Name})
	}
	for _, f := range task.Followers {
		out.Followers = append(out.Followers, &Ref{ID: f.ID, Name: f.Name})
	}

	return out
}

// NewAll converts a slice of tasks into the canonical shape. The result is never
// nil, so it serializes as [] rather than null.
func NewAll(tasks []*asana.Task) []*Task {
	out := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, New(task))
	}
	return out
}

// Write serializes one task.
func Write(w io.Writer, task *asana.Task) error {
	return Encode(w, New(task))
}

// WriteAll serializes a list of tasks as a JSON array.
func WriteAll(w io.Writer, tasks []*asana.Task) error {
	return Encode(w, NewAll(tasks))
}

// Encode writes indented JSON with HTML escaping turned off.
//
// Escaping would render html_notes as <a href=... — semantically identical,
// and unreadable, which defeats the point of recording the rich-text form.
// Nothing is written unless encoding succeeds, so a failure cannot leave partial
// JSON on the stream.
func Encode(w io.Writer, value any) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("failed to render JSON: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}
