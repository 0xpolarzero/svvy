package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventResourceListChanged(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		event := Event{
			ID: "test-id",
			Msg: &EventResourceListChanged{
				ServerName: "test-server",
			},
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var unmarshaled Event
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, "test-id", unmarshaled.ID)
		assert.Equal(t, "resource_list_changed", unmarshaled.Msg.EventType())
		
		resourceEvent, ok := unmarshaled.Msg.(*EventResourceListChanged)
		require.True(t, ok)
		assert.Equal(t, "test-server", resourceEvent.ServerName)
	})
}
