package metabasemcp

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCollectionsTool(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	tool := &ListCollectionsTool{}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check the response is a string
	output, ok := result.(string)
	require.True(t, ok, "Result should be a string")

	// Verify the output contains table headers
	assert.Contains(t, output, "| ID | Name | Description | Location | Type |")
	assert.Contains(t, output, "| --- | --- | --- | --- | --- |")

	// Should contain at least some collections
	assert.Contains(t, output, "collections found")

	// Check for specific collections that should exist
	lines := strings.Split(output, "\n")
	foundRoot := false

	for _, line := range lines {
		if strings.Contains(line, "| root |") && strings.Contains(line, "Root") {
			foundRoot = true
			t.Log("Found root collection")
		}
		if strings.Contains(line, "Personal") {
			t.Log("Found a Personal collection")
		}
	}

	// Root collection should always exist
	assert.True(t, foundRoot, "Should have found root collection")

	t.Logf("List collections output:\n%s", output)
}
