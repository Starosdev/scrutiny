# Shared startup gate for the omnibus run-once services under /etc/services.d.
#
# Sourced, not executed. Only the all-in-one omnibus image ships these services;
# the standalone collector images gate in their own entrypoint scripts instead.
#
# scrutiny_startup_disabled succeeds when the supplied value is an explicit
# false. An unset or empty value is not a false, which keeps the long-standing
# startup run for deployments that never set the variable.
#
# The accepted spellings match strconv.ParseBool, so the all-in-one image and
# collector/pkg/omnibus/config.go agree on what a boolean is. Surrounding quotes
# and whitespace are stripped first because docker --env-file preserves them,
# which is the same reason /etc/cont-init.d/50-cron-config strips quotes from
# cron values.
scrutiny_startup_normalize() {
    local value="${1-}"
    value="${value//\"/}"
    value="${value//\'/}"
    printf '%s' "${value}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'
}

scrutiny_startup_disabled() {
    case "$(scrutiny_startup_normalize "${1-}")" in
        false|f|0) return 0 ;;
        *) return 1 ;;
    esac
}

# scrutiny_startup_enabled is the mirror image: it succeeds only on an explicit
# true. The optional collectors use it so that "is this collector enabled at all"
# and "was the startup run declined" share one notion of a boolean.
scrutiny_startup_enabled() {
    case "$(scrutiny_startup_normalize "${1-}")" in
        true|t|1) return 0 ;;
        *) return 1 ;;
    esac
}
