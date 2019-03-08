package workflow

import (
	"github.com/go-gorp/gorp"
	"github.com/ovh/cds/sdk"
)

var nodeNamePattern = sdk.NamePatternRegex

// CountPipeline Count the number of workflow that use the given pipeline
func CountPipeline(db gorp.SqlExecutor, pipelineID int64) (bool, error) {
	query := `SELECT count(1) FROM workflow_node WHERE pipeline_id= $1`
	nbWorkfow := -1
	err := db.QueryRow(query, pipelineID).Scan(&nbWorkfow)
	return nbWorkfow != 0, err
}
