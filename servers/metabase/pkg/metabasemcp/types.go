package metabasemcp

// Database represents a Metabase database
type Database struct {
	ID                       int                    `json:"id"`
	Name                     string                 `json:"name"`
	Engine                   string                 `json:"engine"`
	Details                  map[string]interface{} `json:"details"`
	IsFullSync               bool                   `json:"is_full_sync"`
	IsSample                 bool                   `json:"is_sample"`
	AutoRunQueries           bool                   `json:"auto_run_queries"`
	Description              string                 `json:"description"`
	CreatedAt                string                 `json:"created_at"`
	UpdatedAt                string                 `json:"updated_at"`
	NativePermissions        string                 `json:"native_permissions"`
	InitialSyncStatus        string                 `json:"initial_sync_status"`
	IsOnDemand               bool                   `json:"is_on_demand"`
	MetadataSyncSchedule     string                 `json:"metadata_sync_schedule"`
	CacheFieldValuesSchedule string                 `json:"cache_field_values_schedule"`
	Timezone                 string                 `json:"timezone"`
	Settings                 map[string]interface{} `json:"settings"`
	Features                 []string               `json:"features"`
}

// Card represents a Metabase card (saved question)
type Card struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DatabaseID  int    `json:"database_id"`
	TableID     int    `json:"table_id"`
	QueryType   string `json:"query_type"`
}

// Dashboard represents a Metabase dashboard
type Dashboard struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cards       []Card `json:"ordered_cards"`
}

// Table represents a Metabase table
type Table struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DatabaseID  int    `json:"db_id"`
	Schema      string `json:"schema"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}

// Field represents a table field/column
type Field struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	DisplayName  string  `json:"display_name"`
	TableID      int     `json:"table_id"`
	Type         string  `json:"type"`
	BaseType     string  `json:"base_type"`
	SemanticType *string `json:"semantic_type"`
	Active       bool    `json:"active"`
	Description  string  `json:"description"`
	DatabaseType string  `json:"database_type"`
}

// Collection represents a Metabase collection
type Collection struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	ParentID    *int   `json:"parent_id"`
}

// User represents a Metabase user
type User struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsActive  bool   `json:"is_active"`
	IsAdmin   bool   `json:"is_superuser"`
}

// QueryResponse represents the response from a query execution
type QueryResponse struct {
	Status      string                 `json:"status"`
	Data        QueryData              `json:"data"`
	DatabaseID  int                    `json:"database_id"`
	RowCount    int                    `json:"row_count"`
	RunningTime int                    `json:"running_time"`
	StartedAt   string                 `json:"started_at"`
	Context     string                 `json:"context"`
	JsonQuery   map[string]interface{} `json:"json_query"`
	Error       string                 `json:"error,omitempty"`
	ErrorType   string                 `json:"error_type,omitempty"`
	Class       string                 `json:"class,omitempty"`
	State       string                 `json:"state,omitempty"`
	StackTrace  []string               `json:"stacktrace,omitempty"`
}

// QueryData contains the actual query results and metadata
type QueryData struct {
	Rows            [][]interface{}        `json:"rows"`
	Cols            []ColumnMetadata       `json:"cols"`
	ResultsMetadata ResultsMetadata        `json:"results_metadata"`
	NativeForm      map[string]interface{} `json:"native_form"`
	ResultsTimezone string                 `json:"results_timezone"`
	Insights        []interface{}          `json:"insights"`
}

// ColumnMetadata describes a column in the query result
type ColumnMetadata struct {
	Name          string        `json:"name"`
	DisplayName   string        `json:"display_name"`
	BaseType      string        `json:"base_type"`
	EffectiveType string        `json:"effective_type"`
	FieldRef      []interface{} `json:"field_ref"`
	Source        string        `json:"source"`
}

// ResultsMetadata contains detailed metadata about the query results
type ResultsMetadata struct {
	Columns []ColumnDetails `json:"columns"`
}

// ColumnDetails contains detailed information about a column
type ColumnDetails struct {
	Name          string                 `json:"name"`
	DisplayName   string                 `json:"display_name"`
	BaseType      string                 `json:"base_type"`
	EffectiveType string                 `json:"effective_type"`
	SemanticType  *string                `json:"semantic_type"`
	FieldRef      []interface{}          `json:"field_ref"`
	Fingerprint   map[string]interface{} `json:"fingerprint"`
}
