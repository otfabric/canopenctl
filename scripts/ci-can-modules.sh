#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# ci-can-modules.sh: load the CAN modules canopenctl's tests need (can,
# can-raw, vcan) on a GitHub-hosted Ubuntu runner, installing the kernel's
# linux-modules-extra package when the runner lacks it, and create vcan0.
set -euo pipefail

kernel=$(uname -r)
if ! sudo modprobe -a can can-raw vcan 2>/dev/null; then
    echo "Installing linux-modules-extra-${kernel}"
    sudo apt-get update -q
    sudo apt-get install -y -q "linux-modules-extra-${kernel}"
    sudo modprobe -a can can-raw vcan
fi
echo "can, can-raw, vcan loaded on ${kernel}"
iface=${1:-vcan0}
if ! ip link show "$iface" >/dev/null 2>&1; then
    sudo ip link add "$iface" type vcan
fi
sudo ip link set "$iface" mtu 72 up
ip -details link show "$iface"
