-- +migrate Up
DELETE from workflow_node_run_job where workflow_node_run_id IN (3976508, 3976509, 3976510,3976511);

-- +migrate Down

