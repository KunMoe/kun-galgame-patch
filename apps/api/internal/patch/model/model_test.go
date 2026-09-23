package model_test

import (
	"encoding/json"
	"testing"

	"kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/userclient"

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

func TestNewPatchUser_CarriesCosmeticsOnTheWire(t *testing.T) {
	assert.Nil(t, model.NewPatchUser(nil))

	bare, err := json.Marshal(model.NewPatchUser(&userclient.Brief{ID: 1, Name: "alice"}))
	require.NoError(t, err)
	assert.NotContains(t, string(bare), "cosmetics")

	worn, err := json.Marshal(model.NewPatchUser(&userclient.Brief{ID: 2, Name: "bob", Cosmetics: &userclient.Cosmetics{
		AvatarFrame: &userclient.Decoration{ItemID: 3, Name: "sakura", StaticURL: "https://img/d/a.png"},
	}}))
	require.NoError(t, err)
	assert.Contains(t, string(worn), `"cosmetics":{"avatar_frame":{"item_id":3,"name":"sakura","static_url":"https://img/d/a.png"}}`)
}
