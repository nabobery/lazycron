package testutil

const FixtureValid = `# Daily backup
MAILTO=ops@example.com
0 3 * * * /usr/local/bin/backup-db
# Hourly health check
*/15 * * * * /usr/local/bin/healthcheck --quiet
@daily /usr/local/bin/cleanup-logs
@reboot /usr/local/bin/start-agent
`

const FixtureWithDisabled = `0 3 * * * /usr/local/bin/backup-db
# [lazycron-disabled] */15 * * * * /usr/local/bin/healthcheck --quiet
@daily /usr/local/bin/cleanup-logs
`

const FixtureInvalid = `0 3 * * * /usr/local/bin/backup-db
this is not a valid cron line
* * * /only/three/fields
@daily /usr/local/bin/cleanup-logs
`

const FixtureMixed = `# Environment setup
SHELL=/bin/bash
PATH=/usr/local/bin:/usr/bin:/bin
MAILTO=admin@example.com

# Backup jobs
0 3 * * * /usr/local/bin/backup-db
30 4 * * 0 /usr/local/bin/weekly-report

# Monitoring
CRON_TZ=America/New_York 0 9 * * MON-FRI /usr/local/bin/morning-check

# Disabled by lazycron
# [lazycron-disabled] */5 * * * * /usr/local/bin/noisy-job

# This is just a comment
@reboot /usr/local/bin/start-agent

not a valid line at all
`

const FixtureEmpty = ``

const FixtureOnlyComments = `# This file has no jobs
# Just comments
`

const FixtureEnvOnly = `SHELL=/bin/bash
PATH=/usr/local/bin:/usr/bin:/bin
MAILTO=ops@example.com
`

const FixtureEveryDescriptor = `@every 5m /usr/local/bin/poll-queue
@every 30s /usr/local/bin/heartbeat --fast
`

const FixtureTabSeparated = "CRON_TZ=UTC\t@daily\t/usr/local/bin/tz-daily\n" +
	"TZ=Europe/Berlin\t0 3 * * *\t/usr/local/bin/tz-standard\n" +
	"0\t3\t*\t*\t*\t/usr/local/bin/tab-fields\n"
