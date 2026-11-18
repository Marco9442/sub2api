package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChannelMonitorDisplayGroup is the catalog for the user channel-status page.
// Membership stays on channel_monitors.group_name. This table only stores the
// stable name and the order of sections.
type ChannelMonitorDisplayGroup struct {
	ent.Schema
}

func (ChannelMonitorDisplayGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channel_monitor_display_groups"},
	}
}

func (ChannelMonitorDisplayGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ChannelMonitorDisplayGroup) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(100).
			Unique(),
		field.Int("sort_order").
			Default(0),
	}
}

func (ChannelMonitorDisplayGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sort_order", "id"),
	}
}
