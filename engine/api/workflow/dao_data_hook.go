package workflow

import (
	"database/sql"
	"fmt"

	"github.com/go-gorp/gorp"

	"github.com/ovh/cds/engine/api/database/gorpmapping"
	"github.com/ovh/cds/sdk"
)

func insertNodeHookData(db gorp.SqlExecutor, w *sdk.Workflow, n *sdk.Node) error {
	if n.Hooks == nil || len(n.Hooks) == 0 {
		return nil
	}

	hookToKeep := make([]sdk.NodeHook, 0)
	for i := range n.Hooks {
		h := &n.Hooks[i]
		h.NodeID = n.ID

		model := w.HookModels[h.HookModelID]
		if model.Name == sdk.RepositoryWebHookModelName && n.Context.ApplicationID == 0 {
			continue
		}

		//Configure the hook
		h.Config[sdk.HookConfigProject] = sdk.WorkflowNodeHookConfigValue{
			Value:        w.ProjectKey,
			Configurable: false,
		}

		h.Config[sdk.HookConfigWorkflow] = sdk.WorkflowNodeHookConfigValue{
			Value:        w.Name,
			Configurable: false,
		}

		h.Config[sdk.HookConfigWorkflowID] = sdk.WorkflowNodeHookConfigValue{
			Value:        fmt.Sprint(w.ID),
			Configurable: false,
		}

		if model.Name == sdk.RepositoryWebHookModelName || model.Name == sdk.GitPollerModelName {
			if n.Context.ApplicationID == 0 || w.Applications[n.Context.ApplicationID].RepositoryFullname == "" || w.Applications[n.Context.ApplicationID].VCSServer == "" {
				return sdk.WrapError(sdk.ErrForbidden, "insertNodeHookData> Cannot create a git poller or repository webhook on an application without a repository")
			}
			h.Config["vcsServer"] = sdk.WorkflowNodeHookConfigValue{
				Value:        w.Applications[n.Context.ApplicationID].VCSServer,
				Configurable: false,
			}
			h.Config["repoFullName"] = sdk.WorkflowNodeHookConfigValue{
				Value:        w.Applications[n.Context.ApplicationID].RepositoryFullname,
				Configurable: false,
			}
		}

		dbHook := dbNodeHookData(*h)
		if err := db.Insert(&dbHook); err != nil {
			return sdk.WrapError(err, "insertNodeHookData> Unable to insert workflow node hook")
		}
		h.ID = dbHook.ID

		hookToKeep = append(hookToKeep, *h)
	}

	n.Hooks = hookToKeep
	return nil
}

// PostInsert is a db hook
func (h *dbNodeHookData) PostInsert(db gorp.SqlExecutor) error {
	return h.PostUpdate(db)
}

// PostUpdate is a db hook
func (h *dbNodeHookData) PostUpdate(db gorp.SqlExecutor) error {
	config, errC := gorpmapping.JSONToNullString(h.Config)
	if errC != nil {
		return sdk.WrapError(errC, "dbNodeHookData.PostUpdate> Unable to marshall config")
	}

	if _, err := db.Exec("UPDATE w_node_hook SET config = $1 WHERE id = $2", config, h.ID); err != nil {
		return sdk.WrapError(err, "dbNodeHookData.PostUpdate> Unable to update config")
	}
	return nil
}

// LoadHookByUUID loads a single hook
func LoadHookByUUID(db gorp.SqlExecutor, uuid string) (*sdk.NodeHook, error) {
	query := `
		SELECT *
			FROM w_node_hook
		WHERE uuid = $1`

	res := dbNodeHookData{}
	if err := db.SelectOne(&res, query, uuid); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, sdk.WithStack(err)
	}

	if err := res.PostGet(db); err != nil {
		return nil, sdk.WrapError(err, "cannot load postget")
	}
	wNodeHook := sdk.NodeHook(res)

	return &wNodeHook, nil
}

//PostGet is a db hook
func (r *dbNodeHookData) PostGet(db gorp.SqlExecutor) error {
	var res = struct {
		Config sql.NullString `db:"config"`
	}{}
	if err := db.SelectOne(&res, "select config from w_node_hook where id = $1", r.ID); err != nil {
		return err
	}

	conf := sdk.WorkflowNodeHookConfig{}

	if err := gorpmapping.JSONNullString(res.Config, &conf); err != nil {
		return err
	}

	r.Config = conf
	return nil
}

// LoadAllHooks returns all hooks
func LoadAllHooks(db gorp.SqlExecutor) ([]sdk.NodeHook, error) {
	res := []dbNodeHookData{}
	if _, err := db.Select(&res, "select * from workflow_node_hook"); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, sdk.WrapError(err, "LoadAllHooks")
	}

	nodes := make([]sdk.NodeHook, 0, len(res))
	for i := range res {
		if err := res[i].PostGet(db); err != nil {
			return nil, sdk.WrapError(err, "LoadAllHooks")
		}
		nodes = append(nodes, sdk.NodeHook(res[i]))
	}

	return nodes, nil
}
