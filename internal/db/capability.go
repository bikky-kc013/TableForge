package db

// Capability replaces phpPgAdmin's per-version subclass overrides
// (Postgres.php through Postgres13.php). On connect we query
// SHOW server_version_num once and derive booleans from it.
// Minimum supported PG is 12 (released 2019); most forks before 9.x
// are irrelevant. Supported 2026: 14–18.
type Capability struct {
	VersionNum int // e.g. 140005 for 14.5
	Major      int // e.g. 14
}

// Derive returns Capability from server_version_num.
func Derive(versionNum int) Capability {
	return Capability{
		VersionNum: versionNum,
		Major:      versionNum / 10000,
	}
}

// The has* methods mirror Postgres.php hasFoo() but are single canonical predicates.

func (c Capability) HasRoles() bool { return c.Major >= 8 } // roles vs users/groups pre-8.1
func (c Capability) HasTablespaces() bool { return c.Major >= 8 }
func (c Capability) HasAutovacuum() bool { return c.Major >= 8 }
func (c Capability) HasPreparedXacts() bool { return c.Major >= 8 }
func (c Capability) HasServerAdminFuncs() bool { return c.Major >= 8 }
func (c Capability) HasAlterDatabaseOwner() bool { return c.Major >= 8 }
func (c Capability) HasCreateTableLike() bool { return c.Major >= 8 }
func (c Capability) HasRecluster() bool { return c.Major >= 8 }
func (c Capability) HasConcurrentIndexBuild() bool { return c.Major >= 8 } // 8.2+
func (c Capability) HasFTS() bool { return c.Major >= 8 } // tsearch2 integrated 8.3
func (c Capability) HasEnumTypes() bool { return c.Major >= 8 } // 8.3
func (c Capability) HasVirtualTransactionId() bool { return c.Major >= 8 } // 8.4+
func (c Capability) HasAlterSequenceStart() bool { return c.Major >= 8 } // 8.4+?
func (c Capability) HasDomainConstraints() bool { return c.Major >= 9 }
func (c Capability) HasAlterDomains() bool { return c.Major >= 9 }
func (c Capability) HasFunctionAlterOwner() bool { return c.Major >= 9 }
func (c Capability) HasFunctionAlterSchema() bool { return c.Major >= 9 }
func (c Capability) HasGrantOption() bool { return c.Major >= 9 }
func (c Capability) HasQueryCancel() bool { return c.Major >= 9 }
func (c Capability) HasQueryKill() bool { return c.Major >= 9 }
func (c Capability) HasServerOids() bool { return c.Major <= 11 } // show_oids removed 12+
func (c Capability) HasByteaHexDefault() bool { return c.Major >= 9 }
func (c Capability) HasForceReindex() bool { return false } // deprecated ; use REINDEX
func (c Capability) HasDatabaseCollation() bool { return c.Major >= 9 }
func (c Capability) HasMagicTypes() bool { return true } // SERIAL/BIGSERIAL
func (c Capability) HasDisableTriggers() bool { return true } // ALTER TABLE DISABLE TRIGGER

// Version-specific feature gates relevant for UI/SQL branching:

func (c Capability) SupportsJSONB() bool { return c.Major >= 9 && c.VersionNum >= 90400 }
func (c Capability) SupportsJSON() bool { return c.Major >= 9 && c.VersionNum >= 90200 }
func (c Capability) SupportsNativePartitioning() bool { return c.Major >= 10 }
func (c Capability) SupportsGeneratedColumns() bool { return c.Major >= 12 }
func (c Capability) SupportsProcedures() bool { return c.Major >= 11 }
func (c Capability) SupportsPublicationSubscription() bool { return c.Major >= 10 }
func (c Capability) SupportsIdentityColumns() bool { return c.Major >= 10 }

func (c Capability) HasAlterTableSchema() bool { return true }
func (c Capability) HasAlterSchema() bool { return true }
func (c Capability) HasAlterSchemaOwner() bool { return true }
func (c Capability) HasAlterSequenceSchema() bool { return true }
func (c Capability) HasAlterColumnType() bool { return true }
func (c Capability) HasAlterAggregate() bool { return true }

// Activity / stat column name differences:
// pg_stat_activity.procpid -> pid in 9.2
func (c Capability) ActivityPIDColumn() string {
	if c.VersionNum >= 90200 {
		return "pid"
	}
	return "procpid"
}

// pg_stat_activity.query vs current_query
func (c Capability) ActivityQueryColumn() string {
	if c.VersionNum >= 90200 {
		return "query"
	}
	return "current_query"
}

// For help base URL: include major version string
func (c Capability) HelpVersion() string {
	return string(rune('0'+c.Major)) // simplified; caller formats with fmt.Sprintf
}
