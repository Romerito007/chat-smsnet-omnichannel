package models

// PipelineStage is the embedded BSON sub-document for one Kanban column.
type PipelineStage struct {
	ID     string `bson:"id"`
	Name   string `bson:"name"`
	Order  int    `bson:"order"`
	IsWon  bool   `bson:"is_won,omitempty"`
	IsLost bool   `bson:"is_lost,omitempty"`
	Color  string `bson:"color,omitempty"`
}

// Pipeline is the BSON document for a tenant's sales funnel. Stages are embedded
// (a pipeline carries few stages and is always read whole).
type Pipeline struct {
	Base      `bson:",inline"`
	Name      string          `bson:"name"`
	IsDefault bool            `bson:"is_default,omitempty"`
	Stages    []PipelineStage `bson:"stages,omitempty"`
}
