// Source-only reference predicate for one parser-extracted PRAGMA statement.
// This is neither a migration-file parser nor a Host runtime validator.
const DEFER_FOREIGN_KEYS_ASSIGNMENT =
  /^[\t\n\v\f\r ]*PRAGMA[\t\n\v\f\r ]+defer_foreign_keys[\t\n\v\f\r ]*=[\t\n\v\f\r ]*(?:ON|OFF|TRUE|FALSE|1|0)[\t\n\v\f\r ]*;?[\t\n\v\f\r ]*$/i;

export function isAllowedMigrationPragma(statement) {
  return (
    typeof statement === "string" &&
    !/[^\u0000-\u007f]/u.test(statement) &&
    DEFER_FOREIGN_KEYS_ASSIGNMENT.test(statement)
  );
}
