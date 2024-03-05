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

# Function to print usage
usage() {
  echo "Usage:"
  echo "$0 command NAME [command-options]"
  echo "Commands:"
  echo "  backup          Create backup"
  echo "    Options:"
  echo "      -n <namespace>             Namespace in which the backup should operate"
  echo "      -i <include-namespaces>    Namespaces to include in the backup"
  echo "      -s <selector>              Label selector for resources to back up"
  echo "      -r <include-resources>     Resources to include in the backup"
  echo "      -l <snapshot-location>     Locations where volume snapshots should be stored"
  echo "      -v                         Verify backup completion"
  echo "  delete-backup   Delete backup"
  echo "    Options:"
  echo "      -n <namespace>             Namespace in which Kasten should operate"
  echo "  restore         Restore a backup"
  echo "    Options:"
  echo "      -n <namespace>             Namespace in which the backup resides"
  echo "      -f <from-backup>           Backup to restore from"
  echo "      -v                         Verify restore completion"
  exit 1
}

# Function to create backup
create_backup() {
  local backup_name=$1
  shift
  local namespace=""
  local include_ns=""
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

  if [ -n "$selector" ]; then
      # TODO: implement your backup with a label selector given like app=myapp
  fi
  if [ -n "$include_resources" ]; then
      # TODO: implement your backup with include resource filter given like virtualmachines,datavolumes
  fi
  # if [ -n "$snapshot_location" ]; then
  #     # currenly
  # fi

  # TODO: implement your backup

  if $verify; then
    verify_backup_completion "$backup_name" "$include_ns"
  fi
}

# Function to verify backup completion
verify_backup_completion() {
  local backup_name=$1
  local namespace=$2
  local timeout=120
  local elapsed_time=0
  echo "Verifying creation of backup $backup_name in namespace $namespace..."

  while [ "$elapsed_time" -lt "$timeout" ]; do
    #TODO: implement your way of checking if backup succeeded/failed

    echo "Waiting for backup $backup_name to reach 'Complete' state, current state: $status..."
    sleep 5
    ((elapsed_time+=5))
  done

  echo "Failed to reach 'Complete' state"
  exit 1
}

# Function to delete backup
delete_backup() {
  local backup_name=$1
  shift
  local namespace=""

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

  #TODO: implement your way of deleting backup all releated objects that might not
  # get deleted when deleteing the test namespace
}

# Function to restore backup
restore_backup() {
  local restore_name=$1
  shift
  local namespace=""
  local from_backup=""
  local verify=false

  # Parse command options
  while getopts "n:f:v" opt; do
    case $opt in
      n)
        namespace=$OPTARG
        ;;
      f)
        from_backup=$OPTARG
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
  if [ ! -f "$RESTORE_ACTION_YAML" ]; then
    echo "Error: YAML file '$RESTORE_ACTION_YAML' not found" >&2
    exit 1
  fi

  # TODO: implement your restore

  if $verify; then
    verify_restore_completion "$restore_name" "$include_ns"
  fi
}

# Function to verify restore completion
verify_restore_completion() {
  local restore_name=$1
  local namespace=$2
  local timeout=120
  local elapsed_time=0
  echo "Verifying restore $restore_name in namespace $namespace..."

  while [ "$elapsed_time" -lt "$timeout" ]; do
    #TODO: implement your way of checking if restore succeeded/failed

    echo "Waiting for restore $restore_name to reach 'Complete' state, current state: $status..."
    sleep 5
    ((elapsed_time+=5))
  done

  echo "Failed to reach 'Complete' state!"
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
    verify_backup_completion "$@"
    ;;
  *)
    echo "Invalid command: $command"
    usage
    ;;
esac

echo "Exiting..."
