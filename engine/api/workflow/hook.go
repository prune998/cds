package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fsamin/go-dump"
	"github.com/go-gorp/gorp"

	"github.com/ovh/cds/engine/api/cache"
	"github.com/ovh/cds/engine/api/repositoriesmanager"
	"github.com/ovh/cds/engine/api/services"
	"github.com/ovh/cds/sdk"
	"github.com/ovh/cds/sdk/log"
)

// HookRegistration ensures hooks registration on Hook µService
func HookRegistration(ctx context.Context, db gorp.SqlExecutor, store cache.Store, oldW *sdk.Workflow, wf sdk.Workflow, p *sdk.Project) error {
	var hookToUpdate map[string]sdk.NodeHook
	var hookToDelete map[string]sdk.NodeHook
	if oldW != nil {
		hookToUpdate, hookToDelete = mergeAndDiffHook(oldW.WorkflowData.GetHooks(), wf.WorkflowData.GetHooks())
	} else {
		hookToUpdate = wf.WorkflowData.GetHooks()
	}

	if len(hookToUpdate) > 0 {
		//Push the hook to hooks µService
		//Load service "hooks"
		srvs, err := services.FindByType(db, services.TypeHooks)
		if err != nil {
			return sdk.WrapError(err, "Unable to get services dao")
		}

		// Update in VCS
		for i := range hookToUpdate {
			h := hookToUpdate[i]
			if oldW != nil && wf.Name != oldW.Name {
				configValue := h.Config[sdk.HookConfigWorkflow]
				configValue.Value = wf.Name
				h.Config[sdk.HookConfigWorkflow] = configValue
				hookToUpdate[i] = h
			}
		}

		//Perform the request on one off the hooks service
		if len(srvs) < 1 {
			return sdk.WrapError(fmt.Errorf("No hooks service available, please try again"), "Unable to get services dao")
		}

		// Update scheduler payload
		for i := range hookToUpdate {
			h := hookToUpdate[i]

			if h.HookModelName == sdk.SchedulerModelName {
				// Add git.branch in scheduler payload
				if wf.WorkflowData.Node.IsLinkedToRepo(&wf) {
					var payloadValues map[string]string
					if h.Config["payload"].Value != "" {
						var bodyJSON interface{}
						//Try to parse the body as an array
						bodyJSONArray := []interface{}{}
						if err := json.Unmarshal([]byte(h.Config["payload"].Value), &bodyJSONArray); err != nil {
							//Try to parse the body as a map
							bodyJSONMap := map[string]interface{}{}
							if err2 := json.Unmarshal([]byte(h.Config["payload"].Value), &bodyJSONMap); err2 == nil {
								bodyJSON = bodyJSONMap
							}
						} else {
							bodyJSON = bodyJSONArray
						}

						//Go Dump
						e := dump.NewDefaultEncoder()
						e.Formatters = []dump.KeyFormatterFunc{dump.WithDefaultLowerCaseFormatter()}
						e.ExtraFields.DetailedMap = false
						e.ExtraFields.DetailedStruct = false
						e.ExtraFields.DeepJSON = true
						e.ExtraFields.Len = false
						e.ExtraFields.Type = false
						var errDump error
						payloadValues, errDump = e.ToStringMap(bodyJSON)
						if errDump != nil {
							return sdk.WrapError(errDump, "HookRegistration> Cannot dump payload %+v", h.Config["payload"].Value)
						}
					}

					// try get git.branch on defaultPayload
					if payloadValues["git.branch"] == "" {
						defaultPayloadMap, errP := wf.WorkflowData.Node.Context.DefaultPayloadToMap()
						if errP != nil {
							return sdk.WrapError(errP, "HookRegistration> Cannot read node default payload")
						}
						if defaultPayloadMap["WorkflowNodeContextDefaultPayloadVCS.GitBranch"] != "" {
							payloadValues["git.branch"] = defaultPayloadMap["WorkflowNodeContextDefaultPayloadVCS.GitBranch"]
						}
						if defaultPayloadMap["WorkflowNodeContextDefaultPayloadVCS.GitRepository"] != "" {
							payloadValues["git.repository"] = defaultPayloadMap["WorkflowNodeContextDefaultPayloadVCS.GitRepository"]
						}
					}

					// try get git.branch on repo linked
					if payloadValues["git.branch"] == "" {
						defaultPayload, errDefault := DefaultPayload(ctx, db, store, p, &wf)
						if errDefault != nil {
							return sdk.WrapError(errDefault, "HookRegistration> Unable to get default payload")
						}
						dumper := dump.NewDefaultEncoder()
						dumper.ExtraFields.DetailedMap = false
						dumper.ExtraFields.DetailedStruct = false
						dumper.ExtraFields.Len = false
						dumper.ExtraFields.Type = false
						var errDump error
						payloadValues, errDump = dumper.ToStringMap(defaultPayload)
						if errDump != nil {
							return sdk.WrapError(errDump, "HookRegistration> Cannot dump payload %+v", h.Config["payload"].Value)
						}
					}

					payloadStr, errM := json.MarshalIndent(&payloadValues, "", "  ")
					if errM != nil {
						return sdk.WrapError(errM, "HookRegistration> Cannot marshal hook config payload : %s", errM)
					}
					pl := h.Config["payload"]
					pl.Value = string(payloadStr)
					h.Config["payload"] = pl
					hookToUpdate[i] = h
				}
			}
		}

		// Create hook on µservice
		code, errHooks := services.DoJSONRequest(ctx, srvs, http.MethodPost, "/task/bulk", hookToUpdate, &hookToUpdate)
		if errHooks != nil || code >= 400 {
			return sdk.WrapError(errHooks, "HookRegistration> Unable to create hooks [%d]", code)
		}

		// Create vcs configuration ( always after hook creation to have webhook URL) + update hook in DB
		for i := range hookToUpdate {
			h := hookToUpdate[i]
			v, ok := h.Config["webHookID"]
			if h.HookModelName == sdk.RepositoryWebHookModelName && h.Config["vcsServer"].Value != "" && (!ok || v.Value == "") {
				if err := createVCSConfiguration(ctx, db, store, p, &h); err != nil {
					return sdk.WrapError(err, "Cannot update vcs configuration")
				}
			}
		}
	}

	if len(hookToDelete) > 0 {
		if err := DeleteHookConfiguration(ctx, db, store, p, hookToDelete); err != nil {
			return sdk.WrapError(err, "Cannot remove hook configuration")
		}
	}
	return nil
}

// DeleteHookConfiguration delete hooks configuration (and their vcs configuration)
func DeleteHookConfiguration(ctx context.Context, db gorp.SqlExecutor, store cache.Store, p *sdk.Project, hookToDelete map[string]sdk.NodeHook) error {
	// Delete from vcs configuration if needed
	for _, h := range hookToDelete {
		if h.HookModelName == sdk.RepositoryWebHookModelName {
			// Call VCS to know if repository allows webhook and get the configuration fields
			projectVCSServer := repositoriesmanager.GetProjectVCSServer(p, h.Config["vcsServer"].Value)
			if projectVCSServer != nil {
				client, errclient := repositoriesmanager.AuthorizedClient(ctx, db, store, projectVCSServer)
				if errclient != nil {
					return sdk.WrapError(errclient, "deleteHookConfiguration> Cannot get vcs client")
				}
				vcsHook := sdk.VCSHook{
					Method:   "POST",
					URL:      h.Config["webHookURL"].Value,
					Workflow: true,
					ID:       h.Config["webHookID"].Value,
				}
				if err := client.DeleteHook(ctx, h.Config["repoFullName"].Value, vcsHook); err != nil {
					log.Error("deleteHookConfiguration> Cannot delete hook on repository %s", err)
				}
				h.Config["webHookID"] = sdk.WorkflowNodeHookConfigValue{
					Value:        vcsHook.ID,
					Configurable: false,
				}
			}
		}
	}

	//Push the hook to hooks µService
	//Load service "hooks"
	srvs, err := services.FindByType(db, services.TypeHooks)
	if err != nil {
		return sdk.WrapError(err, "Unable to get services dao")
	}
	code, errHooks := services.DoJSONRequest(ctx, srvs, http.MethodDelete, "/task/bulk", hookToDelete, nil)
	if errHooks != nil || code >= 400 {
		// if we return an error, transaction will be rollbacked => hook will in database be not anymore on gitlab/bitbucket/github.
		// so, it's just a warn log
		log.Warning("HookRegistration> Unable to delete old hooks [%d]: %s", code, errHooks)
	}
	return nil
}

func createVCSConfiguration(ctx context.Context, db gorp.SqlExecutor, store cache.Store, p *sdk.Project, h *sdk.NodeHook) error {
	// Call VCS to know if repository allows webhook and get the configuration fields
	projectVCSServer := repositoriesmanager.GetProjectVCSServer(p, h.Config["vcsServer"].Value)
	if projectVCSServer == nil {
		return nil
	}

	client, errclient := repositoriesmanager.AuthorizedClient(ctx, db, store, projectVCSServer)
	if errclient != nil {
		return sdk.WrapError(errclient, "createVCSConfiguration> Cannot get vcs client")
	}
	webHookInfo, errWH := repositoriesmanager.GetWebhooksInfos(ctx, client)
	if errWH != nil {
		return sdk.WrapError(errWH, "createVCSConfiguration> Cannot get vcs web hook info")
	}
	if !webHookInfo.WebhooksSupported || webHookInfo.WebhooksDisabled {
		return sdk.WrapError(sdk.ErrForbidden, "createVCSConfiguration> hook creation are forbidden")
	}
	vcsHook := sdk.VCSHook{
		Method:   "POST",
		URL:      h.Config["webHookURL"].Value,
		Workflow: true,
	}
	if err := client.CreateHook(ctx, h.Config["repoFullName"].Value, &vcsHook); err != nil {
		return sdk.WrapError(err, "Cannot create hook on repository: %+v", vcsHook)
	}
	h.Config["webHookID"] = sdk.WorkflowNodeHookConfigValue{
		Value:        vcsHook.ID,
		Configurable: false,
	}
	h.Config["webHookURL"] = sdk.WorkflowNodeHookConfigValue{
		Value:        vcsHook.URL,
		Configurable: false,
		Type:         sdk.HookConfigTypeString,
	}
	h.Config[sdk.HookConfigIcon] = sdk.WorkflowNodeHookConfigValue{
		Value:        webHookInfo.Icon,
		Configurable: false,
		Type:         sdk.HookConfigTypeString,
	}

	return nil
}

func mergeAndDiffHook(oldHooks map[string]sdk.NodeHook, newHooks map[string]sdk.NodeHook) (hookToUpdate map[string]sdk.NodeHook, hookToDelete map[string]sdk.NodeHook) {
	hookToUpdate = make(map[string]sdk.NodeHook)
	hookToDelete = make(map[string]sdk.NodeHook)

	for o := range oldHooks {
		for n := range newHooks {
			if oldHooks[o].Ref == newHooks[n].Ref {
				nh := newHooks[n]
				nh.UUID = oldHooks[o].UUID
				if nh.Config == nil {
					nh.Config = sdk.WorkflowNodeHookConfig{}
				}
				//Useful for RepositoryWebHook
				if webhookID, ok := oldHooks[o].Config["webHookID"]; ok {
					nh.Config["webHookID"] = webhookID
				}
				if oldIcon, ok := oldHooks[o].Config["hookIcon"]; oldHooks[o].HookModelID == newHooks[n].HookModelID && ok {
					nh.Config["hookIcon"] = oldIcon
				}
				newHooks[n] = nh
			}
		}
	}

	for key, hNew := range newHooks {
		hold, ok := oldHooks[key]
		// if new hook
		if !ok || !hNew.Equals(hold) {
			hookToUpdate[key] = newHooks[key]
			continue
		}
	}

	for _, oldH := range oldHooks {
		var exist bool
		for _, newH := range newHooks {
			if oldH.UUID == newH.UUID {
				exist = true
				break
			}
		}
		if !exist {
			hookToDelete[oldH.UUID] = oldH
		}
	}
	return
}

// DefaultPayload returns the default payload for the workflow root
func DefaultPayload(ctx context.Context, db gorp.SqlExecutor, store cache.Store, p *sdk.Project, wf *sdk.Workflow) (interface{}, error) {
	if wf.WorkflowData.Node.Context == nil {
		wf.WorkflowData.Node.Context = &sdk.NodeContext{}
	}

	var defaultPayload interface{}
	var app sdk.Application
	if wf.WorkflowData.Node.Context.ApplicationID != 0 {
		app = wf.Applications[wf.WorkflowData.Node.Context.ApplicationID]
	}

	if app.RepositoryFullname != "" {
		defaultBranch := "master"
		projectVCSServer := repositoriesmanager.GetProjectVCSServer(p, app.VCSServer)
		if projectVCSServer != nil {
			client, errclient := repositoriesmanager.AuthorizedClient(ctx, db, store, projectVCSServer)
			if errclient != nil {
				return wf.WorkflowData.Node.Context.DefaultPayload, sdk.WrapError(errclient, "DefaultPayload> Cannot get authorized client")
			}

			branches, errBr := client.Branches(ctx, app.RepositoryFullname)
			if errBr != nil {
				return wf.WorkflowData.Node.Context.DefaultPayload, sdk.WrapError(errBr, "DefaultPayload> Cannot get branches for %s", app.RepositoryFullname)
			}

			for _, branch := range branches {
				if branch.Default {
					defaultBranch = branch.DisplayID
					break
				}
			}
		}

		defaultPayload = wf.WorkflowData.Node.Context.DefaultPayload
		if !wf.WorkflowData.Node.Context.HasDefaultPayload() {
			structuredDefaultPayload := sdk.WorkflowNodeContextDefaultPayloadVCS{
				GitBranch:     defaultBranch,
				GitRepository: app.RepositoryFullname,
			}
			defaultPayloadBtes, _ := json.Marshal(structuredDefaultPayload)
			if err := json.Unmarshal(defaultPayloadBtes, &defaultPayload); err != nil {
				return nil, err
			}
		} else if defaultPayloadMap, err := wf.WorkflowData.Node.Context.DefaultPayloadToMap(); err == nil && defaultPayloadMap["git.branch"] == "" {
			defaultPayloadMap["git.branch"] = defaultBranch
			defaultPayloadMap["git.repository"] = app.RepositoryFullname
			defaultPayload = defaultPayloadMap
		}
	} else {
		defaultPayload = wf.WorkflowData.Node.Context.DefaultPayload
	}

	return defaultPayload, nil
}
