#!/bin/sh
# SPDX-License-Identifier: MIT
# Regenerates docs/cli.md from the --help of every command.
set -eu
cd "$(dirname "$0")/.."
GOWORK=off go build -o bin/canopenctl .
cmds="monitor decode sdo sdo_read sdo_write sdo_dump nmt heartbeat heartbeat_watch scan
emcy emcy_watch sync time pdo pdo_show pdo_map pdo_watch
eds eds_validate eds_show eds_convert eds_generate dcf dcf_apply dcf_export
cia402 cia402_state cia402_enable cia402_disable cia402_fault-reset cia402_quick-stop cia402_mode cia402_move cia402_velocity
lss lss_switch lss_select lss_inquire lss_configure lss_activate lss_identify lss_fastscan lss_assign-all
mpdo mpdo_configure mpdo_scanner mpdo_dispatcher mpdo_watch
boot cdcf cdcf_convert cdcf_show cdcf_apply store restore program program_download program_control program_status
simulate version completion"
{
	echo "# canopenctl command reference"
	echo
	echo "Generated from \`canopenctl <command> --help\` by scripts/gen-cli-doc.sh; do not edit."
	echo "Exit codes: [exitcodes.md](exitcodes.md). JSON output: [json-output.md](json-output.md)."
	echo
	echo '```text'
	bin/canopenctl --help
	echo '```'
	for c in $cmds; do
		args=$(echo "$c" | tr _ ' ')
		echo
		echo "## canopenctl $args"
		echo
		echo '```text'
		# shellcheck disable=SC2086
		CANOPENCTL_INTERFACE= bin/canopenctl $args --help
		echo '```'
	done
} > docs/cli.md
