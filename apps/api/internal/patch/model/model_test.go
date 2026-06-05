package model_test

import (
	"testing"

	"kun-galgame-patch-api/internal/patch/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONArray_Scan_Nil(t *testing.T) {
	var arr model.JSONArray
	err := arr.Scan(nil)
	require.NoError(t, err)
	assert.Equal(t, model.JSONArray{}, arr)
}

func TestJSONArray_Scan_ValidJSON(t *testing.T) {
	var arr model.JSONArray
	err := arr.Scan([]byte(`["a","b","c"]`))
	require.NoError(t, err)
	assert.Equal(t, model.JSONArray{"a", "b", "c"}, arr)
}

func TestJSONArray_Scan_EmptyArray(t *testing.T) {
	var arr model.JSONArray
	err := arr.Scan([]byte(`[]`))
	require.NoError(t, err)
	assert.Equal(t, model.JSONArray{}, arr)
}

func TestJSONArray_Scan_InvalidType(t *testing.T) {
	var arr model.JSONArray
	err := arr.Scan(12345)
	assert.Error(t, err)
}

// Regression: the pgx driver can return jsonb as a string (e.g. the INSERT ...
// RETURNING scan in ensureLocalPatch). Scan must accept it, not error with
// "failed to unmarshal JSONArray: []" and roll back the patch create.
func TestJSONArray_Scan_StringInput(t *testing.T) {
	var arr model.JSONArray
	require.NoError(t, arr.Scan(`["a","b"]`))
	assert.Equal(t, model.JSONArray{"a", "b"}, arr)
}

func TestJSONArray_Scan_StringEmptyArray(t *testing.T) {
	var arr model.JSONArray
	require.NoError(t, arr.Scan(`[]`))
	assert.Equal(t, model.JSONArray{}, arr)
}

func TestJSONArray_Scan_EmptyAndNull(t *testing.T) {
	for _, in := range []any{"", []byte(nil), []byte("null"), "null"} {
		var arr model.JSONArray
		require.NoError(t, arr.Scan(in))
		assert.Equal(t, model.JSONArray{}, arr)
	}
}

func TestJSONArray_Value_Nil(t *testing.T) {
	var arr model.JSONArray
	val, err := arr.Value()
	require.NoError(t, err)
	assert.Equal(t, "[]", val)
}

func TestJSONArray_Value_NonEmpty(t *testing.T) {
	arr := model.JSONArray{"x", "y"}
	val, err := arr.Value()
	require.NoError(t, err)
	// Should be valid JSON
	assert.Contains(t, string(val.([]byte)), "x")
	assert.Contains(t, string(val.([]byte)), "y")
}

func TestPatch_TableName(t *testing.T) {
	assert.Equal(t, "patch", model.Patch{}.TableName())
}

func TestPatchResource_TableName(t *testing.T) {
	assert.Equal(t, "patch_resource", model.PatchResource{}.TableName())
}

func TestPatchComment_TableName(t *testing.T) {
	assert.Equal(t, "patch_comment", model.PatchComment{}.TableName())
}

func TestUserPatchFavoriteRelation_TableName(t *testing.T) {
	assert.Equal(t, "user_patch_favorite_relation", model.UserPatchFavoriteRelation{}.TableName())
}
