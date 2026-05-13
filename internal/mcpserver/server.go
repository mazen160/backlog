// Package mcpserver implements a minimal MCP (Model Context Protocol) server
// over stdio using JSON-RPC 2.0 with newline-delimited messages.
package mcpserver

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/service"
)

// message is a JSON-RPC 2.0 message.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve runs the MCP stdio server until the reader closes.
func Serve(db *sql.DB, actor models.Actor) {
	srv := &server{
		db:    db,
		actor: actor,
		r:     bufio.NewReader(os.Stdin),
		w:     os.Stdout,
	}
	srv.run()
}

type server struct {
	db    *sql.DB
	actor models.Actor
	r     *bufio.Reader
	w     io.Writer
}

func (s *server) run() {
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			s.send(message{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		s.handle(msg)
	}
}

func (s *server) handle(msg message) {
	ctx := context.Background()
	switch msg.Method {
	case "initialize":
		s.send(message{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
				"serverInfo":      map[string]string{"name": "backlog", "version": "1.0.0"},
			},
		})
	case "initialized":
		// notification, no response
	case "tools/list":
		s.send(message{JSONRPC: "2.0", ID: msg.ID, Result: map[string]interface{}{"tools": tools()}})
	case "tools/call":
		result, err := s.callTool(ctx, msg.Params)
		if err != nil {
			s.send(message{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32603, Message: err.Error()}})
			return
		}
		s.send(message{JSONRPC: "2.0", ID: msg.ID, Result: result})
	default:
		s.send(message{JSONRPC: "2.0", ID: msg.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
	}
}

func (s *server) send(msg message) {
	msg.JSONRPC = "2.0"
	data, _ := json.Marshal(msg)
	fmt.Fprintf(s.w, "%s\n", data)
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *server) callTool(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params")
	}

	var args map[string]interface{}
	if len(p.Arguments) > 0 {
		json.Unmarshal(p.Arguments, &args) //nolint:errcheck
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	planSvc := service.NewPlanService(s.db)
	labelSvc := service.NewLabelService(s.db)
	taskSvc := service.NewTaskService(s.db, planSvc, labelSvc)
	projSvc := service.NewProjectService(s.db)
	memorySvc := service.NewMemoryService(s.db)
	docSvc := service.NewDocService(s.db)

	str := func(k string) string {
		if v, ok := args[k]; ok {
			if sv, ok := v.(string); ok {
				return sv
			}
		}
		return ""
	}
	intArg := func(k string) int {
		if v, ok := args[k]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			}
		}
		return 0
	}

	switch p.Name {
	case "project_list":
		projects, err := projSvc.List(ctx, false)
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(map[string]interface{}{"projects": projects})), nil

	case "task_create":
		priority := intArg("priority")
		if priority == 0 {
			priority = 3
		}
		proj, err := projSvc.GetByAlias(ctx, str("project"))
		if err != nil {
			return nil, err
		}
		createIn := models.CreateTaskInput{
			ProjectID:   proj.ID,
			Title:       str("title"),
			Description: str("description"),
			Type:        models.TaskType(str("type")),
			Status:      models.TaskStatus(str("status")),
			Priority:    priority,
			Actor:       s.actor,
			Source:      str("source"),
			ExternalRef: str("external_ref"),
			ProjectPath: str("project_path"),
		}
		if dd := str("due_date"); dd != "" {
			ts, err := parseMCPDate(dd)
			if err != nil {
				return nil, err
			}
			createIn.DueAt = &ts
		}
		t, err := taskSvc.Create(ctx, createIn)
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(t)), nil

	case "task_list":
		f := models.TaskFilter{
			ProjectAlias: str("project"),
			Status:       models.TaskStatus(str("status")),
			Type:         models.TaskType(str("type")),
			Priority:     intArg("priority"),
			Limit:        50,
		}
		if s := str("search"); s != "" {
			f.Search = s
		}
		tasks, total, err := taskSvc.List(ctx, f)
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(map[string]interface{}{"tasks": tasks, "total": total})), nil

	case "task_show":
		t, err := taskSvc.Get(ctx, str("id"), true, true)
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(t)), nil

	case "task_update":
		id := str("id")
		in := models.UpdateTaskInput{}
		if v := str("title"); v != "" {
			in.Title = &v
		}
		if v := str("description"); v != "" {
			in.Description = &v
		}
		if v := str("status"); v != "" {
			st := models.TaskStatus(v)
			in.Status = &st
		}
		if v := intArg("priority"); v > 0 {
			in.Priority = &v
		}
		if dd := str("due_date"); dd != "" {
			ts, err := parseMCPDate(dd)
			if err != nil {
				return nil, err
			}
			in.DueAt = &ts
		}
		if v := str("project_path"); v != "" {
			in.ProjectPath = &v
		}
		t, err := taskSvc.Update(ctx, id, in, s.actor)
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(t)), nil

	case "task_move":
		t, err := taskSvc.Move(ctx, str("id"), models.TaskStatus(str("status")), s.actor)
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(t)), nil

	case "plan_add":
		resolvedTaskID, err := taskSvc.ResolveRef(ctx, str("task_id"))
		if err != nil {
			return nil, err
		}
		pl, err := planSvc.Create(ctx, models.CreatePlanInput{
			TaskID: resolvedTaskID,
			Title:  str("title"),
			Body:   str("body"),
			Source: str("source"),
			Actor:  s.actor,
		})
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(pl)), nil

	case "plan_update":
		pl, err := planSvc.Update(ctx, str("plan_id"), models.UpdatePlanInput{
			Title:      str("title"),
			Body:       str("body"),
			ChangeNote: str("change_note"),
			Actor:      s.actor,
		})
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(pl)), nil

	case "plan_history":
		versions, err := planSvc.History(ctx, str("plan_id"))
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(map[string]interface{}{"versions": versions})), nil

	case "comment_add":
		resolvedCommentTaskID, err := taskSvc.ResolveRef(ctx, str("task_id"))
		if err != nil {
			return nil, err
		}
		c, err := service.NewCommentService(s.db).Create(ctx, models.CreateCommentInput{
			TaskID: resolvedCommentTaskID,
			Body:   str("body"),
			Actor:  s.actor,
		})
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(c)), nil

	case "memory_add":
		proj, err := projSvc.GetByAlias(ctx, str("project"))
		if err != nil {
			return nil, err
		}
		m, err := memorySvc.Add(ctx, models.CreateMemoryInput{
			ProjectID: proj.ID,
			Body:      str("body"),
			Tags:      str("tags"),
			Actor:     s.actor,
		})
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(m)), nil

	case "memory_list":
		entries, err := memorySvc.List(ctx, str("project"), str("tag"))
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(map[string]interface{}{"entries": entries})), nil

	case "doc_add":
		d, err := docSvc.Create(ctx, str("project"), models.CreateDocInput{
			Title: str("title"),
			Body:  str("body"),
			Actor: s.actor,
		})
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(d)), nil

	case "doc_list":
		docs, err := docSvc.List(ctx, str("project"))
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(map[string]interface{}{"docs": docs})), nil

	case "doc_show":
		d, err := docSvc.Get(ctx, str("id"))
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(d)), nil

	case "doc_update":
		d, err := docSvc.Update(ctx, str("id"), models.UpdateDocInput{
			Title:      str("title"),
			Body:       str("body"),
			ChangeNote: str("change_note"),
			Actor:      s.actor,
		})
		if err != nil {
			return nil, err
		}
		return contentText(toJSON(d)), nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", p.Name)
	}
}

func contentText(s string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": s}},
	}
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func tools() []map[string]interface{} {
	return []map[string]interface{}{
		tool("project_list", "List all projects", props()),
		tool("task_create", "Create a new task", props(
			req("project", "string", "project alias"),
			req("title", "string", "task title"),
			opt("description", "string", "task description"),
			opt("type", "string", "task|bug|issue|improvement|feature|vulnerability|chore|spike|bucket-list"),
			opt("status", "string", "todo|doing|done"),
			opt("priority", "integer", "1-5 (1=highest)"),
			opt("source", "string", "source of this task"),
			opt("external_ref", "string", "external reference URL or ID"),
			opt("due_date", "string", "due date (YYYY-MM-DD or RFC3339)"),
			opt("project_path", "string", "file path or URL for relevant code location (e.g. internal/handlers/search.go:84)"),
		)),
		tool("task_list", "List tasks", props(
			opt("project", "string", "filter by project alias"),
			opt("status", "string", "todo|doing|done"),
			opt("type", "string", "task type filter"),
			opt("priority", "integer", "priority filter"),
			opt("search", "string", "full-text search"),
		)),
		tool("task_show", "Show task details with plans and comments", props(req("id", "string", "task ref: TASK-N, bare integer, or ULID"))),
		tool("task_update", "Update a task", props(
			req("id", "string", "task ref: TASK-N, bare integer, or ULID"),
			opt("title", "string", "new title"),
			opt("description", "string", "new description"),
			opt("status", "string", "todo|doing|done"),
			opt("priority", "integer", "1-5"),
			opt("due_date", "string", "due date (YYYY-MM-DD or RFC3339)"),
			opt("project_path", "string", "file path or URL for relevant code location"),
		)),
		tool("task_move", "Move task to a status", props(
			req("id", "string", "task ref: TASK-N, bare integer, or ULID"),
			req("status", "string", "todo|doing|done"),
		)),
		tool("plan_add", "Add a plan to a task", props(
			req("task_id", "string", "task ref: TASK-N, bare integer, or ULID"),
			req("title", "string", "plan title"),
			req("body", "string", "plan body (markdown)"),
			opt("source", "string", "plan source"),
		)),
		tool("plan_update", "Update a plan (creates a new version)", props(
			req("plan_id", "string", "plan ID"),
			req("title", "string", "new title"),
			req("body", "string", "new body"),
			opt("change_note", "string", "reason for change"),
		)),
		tool("plan_history", "Get version history of a plan", props(req("plan_id", "string", "plan ID"))),
		tool("comment_add", "Add a comment to a task", props(
			req("task_id", "string", "task ref: TASK-N, bare integer, or ULID"),
			req("body", "string", "comment body"),
		)),
		tool("memory_add", "Add a memory entry to a project", props(
			req("project", "string", "project alias"),
			req("body", "string", "memory body (free-form text or markdown)"),
			opt("tags", "string", "comma-separated tags for filtering"),
		)),
		tool("memory_list", "List memory entries for a project (newest first)", props(
			req("project", "string", "project alias"),
			opt("tag", "string", "filter by tag"),
		)),
		tool("doc_add", "Add a doc to a project", props(
			req("project", "string", "project alias"),
			req("title", "string", "doc title"),
			req("body", "string", "doc body (markdown)"),
		)),
		tool("doc_list", "List docs for a project", props(
			req("project", "string", "project alias"),
		)),
		tool("doc_show", "Show a doc with its current body", props(
			req("id", "string", "doc ID"),
		)),
		tool("doc_update", "Update a doc (creates a new version)", props(
			req("id", "string", "doc ID"),
			req("body", "string", "new body (markdown)"),
			opt("title", "string", "new title"),
			opt("change_note", "string", "reason for change"),
		)),
	}
}

func tool(name, desc string, inputSchema map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"name": name, "description": desc, "inputSchema": inputSchema}
}

func props(fields ...map[string]interface{}) map[string]interface{} {
	properties := map[string]interface{}{}
	var required []string
	for _, f := range fields {
		n := f["name"].(string)
		properties[n] = map[string]string{"type": f["type"].(string), "description": f["desc"].(string)}
		if f["required"].(bool) {
			required = append(required, n)
		}
	}
	s := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func req(name, typ, desc string) map[string]interface{} {
	return map[string]interface{}{"name": name, "type": typ, "desc": desc, "required": true}
}
func opt(name, typ, desc string) map[string]interface{} {
	return map[string]interface{}{"name": name, "type": typ, "desc": desc, "required": false}
}

func parseMCPDate(s string) (int64, error) {
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UnixNano(), nil
		}
	}
	return 0, fmt.Errorf("invalid date %q (use YYYY-MM-DD or RFC3339)", s)
}
