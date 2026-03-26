#!/bin/bash

# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2024 Red Hat, Inc.

# Set variables
VELERO_CLI=${VELERO_CLI:-velero}

# Dump describe --details for a failed backup or restore to aid debugging
dump_backup_details() {
  local backup_name=$1
  local namespace=$2
  echo "=== velero backup describe --details ==="
  $VELERO_CLI backup describe "$backup_name" --namespace "$namespace" --details 2>&1 || true
  echo "=== velero backup logs (last 50 lines) ==="
  $VELERO_CLI backup logs "$backup_name" --namespace "$namespace" 2>&1 | tail -50 || true
  echo "=== end backup details ==="
}

dump_restore_details() {
  local restore_name=$1
  local namespace=$2
  echo "=== velero restore describe --details ==="
  $VELERO_CLI restore describe "$restore_name" --namespace "$namespace" --details 2>&1 || true
  echo "=== velero restore logs (last 50 lines) ==="
  $VELERO_CLI restore logs "$restore_name" --namespace "$namespace" 2>&1 | tail -50 || true
  echo "=== end restore details ==="
}

# Function to print usage
usage() {
  echo "Usage:"
  echo "$0 command NAME [command-options]"
  echo "Commands:"
  echo "  backup          Create backup"
  echo "    Options:"
  echo "      -n <namespace>             Namespace in which Velero should operate (default: velero)"
  echo "      -i <include-namespaces>    Namespaces to include in the backup (default: *)"
  echo "      -s <selector>              Label selector for resources to back up"
  echo "      -r <include-resources>     Resources to include in the backup"
  echo "      -l <snapshot-location>     Locations where volume snapshots should be stored"
  echo "      -v                         Verify backup completion"
  echo "  delete-backup   Delete backup"
  echo "    Options:"
  echo "      -n <namespace>             Namespace in which Velero should operate (default: velero)"
  echo "  restore         Restore a backup"
  echo "    Options:"
  echo "      -n <namespace>             Namespace in which the backup resides (default: velero)"
  echo "      -f <from-backup>           Backup to restore from"
  echo "      -s <selector>              Label selector for resources to restore"
  echo "      -v                         Verify restore completion"
  exit 1
}

# Function to create backup
create_backup() {
  local backup_name=$1
  shift
  local namespace="velero"
  local include_ns="*"
  local selector=""
  local include_resources=""
  local snapshot_location=""
  local verify=false

  # Parse command options
  while getopts "n:i:s:r:l:v" opt; do
    case $opt in
      n)
        namespace=$OPTARG
        ;;
      i)
        include_ns=$OPTARG
        ;;
      s)
        selector=$OPTARG
        ;;
      r)
        include_resources=$OPTARG
        ;;
      l)
        snapshot_location=$OPTARG
        ;;
      v)
        verify=true
        ;;
      \?)
        echo "Invalid option: -$OPTARG" >&2
        usage
        ;;
    esac
  done
  shift $((OPTIND -1))

  if [ -z "$backup_name" ]; then
    echo "Error: Backup name is required."
    usage
  fi

  echo "Creating backup: $backup_name"
  # Construct command
  local backup_cmd="$VELERO_CLI create backup $backup_name --namespace $namespace --include-namespaces $include_ns --wait"

  if [ -n "$selector" ]; then
    backup_cmd="$backup_cmd --selector $selector"
  fi
  if [ -n "$include_resources" ]; then
    backup_cmd="$backup_cmd --include-resources $include_resources"
  fi
  if [ -n "$snapshot_location" ]; then
    backup_cmd="$backup_cmd --volume-snapshot-locations $snapshot_location"
  fi

  # Execute backup command
  echo "Running backup command: $backup_cmd"
  $backup_cmd

  if $verify; then
    verify_backup_completion "$backup_name" "$namespace"
  fi
}

# Function to verify backup completion
verify_backup_completion() {
  local backup_name=$1
  local namespace=$2
  local max_wait=300
  local interval=5
  local elapsed=0

  echo "Waiting for backup to reach Completed state..."

  while [ $elapsed -lt $max_wait ]; do
    local get_backup="$VELERO_CLI backup get $backup_name -n $namespace -o json"
    local backup=$($get_backup 2>/dev/null)
    local backup_phase=$(echo "$backup" | jq -r '.status.phase' 2>/dev/null)

    echo "Current backup phase: $backup_phase (elapsed: ${elapsed}s)"

    if [ "$backup_phase" == "Completed" ]; then
      echo "Backup completed successfully."
      return 0
    fi

    if [ "$backup_phase" == "Failed" ] || [ "$backup_phase" == "PartiallyFailed" ]; then
      echo "Error: Backup reached terminal failure state: $backup_phase"
      dump_backup_details "$backup_name" "$namespace"
      exit 1
    fi

    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "Error: Backup did not complete within ${max_wait}s. Last status: $backup_phase"
  dump_backup_details "$backup_name" "$namespace"
  exit 1
}

# Function to delete backup
delete_backup() {
  local backup_name=$1
  shift
  local namespace="velero"

  # Parse command options
  while getopts "n:" opt; do
    case $opt in
      n)
        namespace=$OPTARG
        ;;
      \?)
        echo "Invalid option: -$OPTARG" >&2
        usage
        ;;
    esac
  done
  shift $((OPTIND -1))

  if [ -z "$backup_name" ]; then
    echo "Error: Backup name is required."
    usage
  fi

  local delete_backup="$VELERO_CLI delete backup $backup_name --confirm --namespace $namespace"
  echo "Deleting backup: $delete_backup"
  $delete_backup
}

# Function to restore backup
restore_backup() {
  local restore_name=$1
  shift
  local namespace="velero"
  local from_backup=""
  local selector=""
  local verify=false

  # Parse command options
  while getopts "n:f:s:v" opt; do
    case $opt in
      n)
        namespace=$OPTARG
        ;;
      f)
        from_backup=$OPTARG
        ;;
      s)
        selector=$OPTARG
        ;;
      v)
        verify=true
        ;;
      \?)
        echo "Invalid option: -$OPTARG" >&2
        usage
        ;;
    esac
  done
  shift $((OPTIND -1))

  if [ -z "$restore_name" ]; then
    echo "Error: Restore name is required."
    usage
  fi

  if [ -z "$from_backup" ]; then
    echo "Error: Backup name to restore from is required."
    usage
  fi

  if [ -n "$selector" ]; then
    local restore_cmd="$VELERO_CLI restore create $restore_name --from-backup $from_backup --namespace $namespace --selector $selector"
    echo "Running restore: $restore_cmd"
    $restore_cmd
    verify_selective_restore_completion "$restore_name" "$namespace"
  else
    local restore_cmd="$VELERO_CLI restore create $restore_name --from-backup $from_backup --namespace $namespace"
    echo "Running restore: $restore_cmd"
    $restore_cmd
    if $verify; then
      verify_restore_completion "$restore_name" "$namespace"
    fi
  fi
}

# Function to verify restore completion
verify_restore_completion() {
  local restore_name=$1
  local namespace=$2
  local max_wait=300
  local interval=5
  local elapsed=0

  echo "Waiting for restore to reach Completed state..."

  while [ $elapsed -lt $max_wait ]; do
    local get_restore="$VELERO_CLI restore get $restore_name -n $namespace -o json"
    local restore=$($get_restore 2>/dev/null)
    local restore_phase=$(echo "$restore" | jq -r '.status.phase' 2>/dev/null)

    echo "Current restore phase: $restore_phase (elapsed: ${elapsed}s)"

    if [ "$restore_phase" == "Completed" ]; then
      echo "Restore completed successfully."
      return 0
    fi

    if [ "$restore_phase" == "Failed" ] || [ "$restore_phase" == "PartiallyFailed" ]; then
      echo "Error: Restore reached terminal failure state: $restore_phase"
      dump_restore_details "$restore_name" "$namespace"
      exit 1
    fi

    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "Error: Restore did not complete within ${max_wait}s. Last status: $restore_phase"
  dump_restore_details "$restore_name" "$namespace"
  exit 1
}

# Function to verify selective restore completion
verify_selective_restore_completion() {
  local restore_name=$1
  local namespace=$2
  local max_wait=300  # 5 minutes — PV patching in Finalizing can take a while
  local interval=5
  local elapsed=0

  echo "Waiting for selective restore to complete..."

  while [ $elapsed -lt $max_wait ]; do
    local get_restore="$VELERO_CLI restore get $restore_name -n $namespace -o json"
    local restore=$($get_restore 2>/dev/null)
    local restore_phase=$(echo "$restore" | jq -r '.status.phase' 2>/dev/null)

    echo "Current restore phase: $restore_phase (elapsed: ${elapsed}s)"

    if [ "$restore_phase" == "Completed" ] || [ "$restore_phase" == "PartiallyFailed" ]; then
      echo "Selective restore completed: $restore_phase"
      return 0
    fi

    if [ "$restore_phase" == "Failed" ]; then
      echo "Error: Selective restore failed: $restore_phase"
      dump_restore_details "$restore_name" "$namespace"
      exit 1
    fi

    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "Error: Selective restore did not complete within ${max_wait}s. Last status: $restore_phase"
  dump_restore_details "$restore_name" "$namespace"
  exit 1
}

# Parse command
command=$1
shift

# Check if command is provided
if [ -z "$command" ]; then
  echo "Error: Command is required."
  usage
fi

# Switch on the command
case $command in
  "backup")
    create_backup "$@"
    ;;
  "delete-backup")
    delete_backup "$@"
    ;;
  "restore")
    restore_backup "$@"
    ;;
  "verify-backup")
    verify_backup_completion "$@"
    ;;
  "verify-restore")
    verify_restore_completion "$@"
    ;;
  *)
    echo "Invalid command: $command"
    usage
    ;;
esac

echo "Exiting..."
