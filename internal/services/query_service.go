package services

import (
	"silobang/internal/constants"
	"silobang/internal/logger"
	"silobang/internal/queries"
)

// QueryService handles query execution operations.
type QueryService struct {
	app    AppState
	logger *logger.Logger
}

// NewQueryService creates a new query service instance.
func NewQueryService(app AppState, log *logger.Logger) *QueryService {
	return &QueryService{
		app:    app,
		logger: log,
	}
}

// QueryRequest represents a request to execute a query.
type QueryRequest struct {
	Params map[string]interface{} `json:"params"`
	Topics []string               `json:"topics"`
}

// ListPresets returns all available query presets.
func (s *QueryService) ListPresets() ([]queries.PresetInfo, error) {
	qc := s.app.GetQueriesConfig()
	if qc == nil {
		return nil, WrapInternalError(nil)
	}
	return qc.ListPresets(), nil
}

// Execute runs a query preset with the given parameters.
func (s *QueryService) Execute(presetName string, req *QueryRequest) (*queries.QueryResult, []string, error) {
	if s.app.GetWorkingDirectory() == "" {
		return nil, nil, ErrNotConfigured
	}

	qc := s.app.GetQueriesConfig()
	if qc == nil {
		return nil, nil, WrapInternalError(nil)
	}

	// Get preset
	preset, err := qc.GetPreset(presetName)
	if err != nil {
		return nil, nil, ErrPresetNotFoundWithName(presetName)
	}

	// Convert params to strings and validate
	var stringParams map[string]string
	if req != nil && req.Params != nil {
		stringParams = queries.ParamsToStrings(req.Params)
	}

	params, err := queries.ValidateParams(preset, stringParams)
	if err != nil {
		return nil, nil, WrapServiceError(constants.ErrCodeMissingParam, err.Error(), err)
	}

	// Get topic databases
	var topicNames []string
	if req != nil {
		topicNames = req.Topics
	}

	topicDBs, validNames, err := s.app.GetTopicDBsForQuery(topicNames)
	if err != nil {
		return nil, nil, WrapServiceError(constants.ErrCodeTopicUnhealthy, err.Error(), err)
	}

	if len(validNames) == 0 {
		// No topics available - return empty result
		return &queries.QueryResult{
			Preset:   presetName,
			RowCount: 0,
			Columns:  []string{},
			Rows:     [][]interface{}{},
		}, validNames, nil
	}

	// Execute query across topics
	result, err := queries.ExecuteCrossTopicQuery(preset, params, topicDBs, validNames)
	if err != nil {
		return nil, nil, WrapQueryError(err)
	}

	result.Preset = presetName

	s.logger.Debug("Executed query %s across %d topics, returned %d rows", presetName, len(validNames), result.RowCount)

	return result, validNames, nil
}
