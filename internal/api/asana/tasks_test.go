package asana

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every clearable field must round-trip to the exact JSON Asana needs to blank
// it — which is null for some fields and an empty string for others. Getting
// this wrong is invisible locally and shows up as "the update did nothing".
func TestUpdateTaskRequest_MarshalJSON_Clear(t *testing.T) {
	tests := []struct {
		name string
		req  *UpdateTaskRequest
		want string
	}{
		{
			name: "no clears behaves exactly as before",
			req: &UpdateTaskRequest{
				TaskBase: TaskBase{Name: "New name"},
			},
			want: `{"name":"New name"}`,
		},
		{
			name: "clear assignee sends an explicit null",
			req:  &UpdateTaskRequest{Clear: []ClearableField{ClearAssignee}},
			want: `{"assignee":null}`,
		},
		{
			name: "clear due date sends an explicit null",
			req:  &UpdateTaskRequest{Clear: []ClearableField{ClearDueDate}},
			want: `{"due_on":null}`,
		},
		{
			// notes is a string field: Asana blanks it with "", not null. Only
			// notes is sent — Asana rejects notes and html_notes together, and
			// blanking notes blanks the description either way.
			name: "clear notes sends an empty string",
			req:  &UpdateTaskRequest{Clear: []ClearableField{ClearNotes}},
			want: `{"notes":""}`,
		},
		{
			name: "a clear coexists with ordinary fields",
			req: &UpdateTaskRequest{
				TaskBase: TaskBase{Name: "Renamed"},
				Clear:    []ClearableField{ClearAssignee},
			},
			want: `{"assignee":null,"name":"Renamed"}`,
		},
		{
			name: "several clears at once",
			req:  &UpdateTaskRequest{Clear: []ClearableField{ClearAssignee, ClearDueDate}},
			want: `{"assignee":null,"due_on":null}`,
		},
		{
			// A clear must win over a value that omitempty would have dropped
			// anyway, and must not be shadowed by the embedded struct.
			name: "clear overrides an empty embedded value",
			req: &UpdateTaskRequest{
				TaskBase: TaskBase{Notes: ""},
				Clear:    []ClearableField{ClearNotes},
			},
			want: `{"notes":""}`,
		},
		{
			name: "completed false is sent, not omitted",
			req: func() *UpdateTaskRequest {
				no := false
				return &UpdateTaskRequest{TaskBase: TaskBase{Completed: &no}}
			}(),
			want: `{"completed":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Marshal()\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

// The Clear list must never reach the wire as a field of its own.
func TestUpdateTaskRequest_MarshalJSON_OmitsClearItself(t *testing.T) {
	got, err := json.Marshal(&UpdateTaskRequest{Clear: []ClearableField{ClearAssignee}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "Clear") || strings.Contains(string(got), `"clear"`) {
		t.Fatalf("payload leaks the Clear list: %s", got)
	}
}

func TestUpdateTaskRequest_MarshalJSON_UnknownField(t *testing.T) {
	_, err := json.Marshal(&UpdateTaskRequest{Clear: []ClearableField{"not_a_field"}})
	if err == nil || !strings.Contains(err.Error(), "not_a_field") {
		t.Fatalf("expected an error naming the unknown field, got %v", err)
	}
}
