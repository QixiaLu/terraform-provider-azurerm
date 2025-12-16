#!/usr/bin/env bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: MPL-2.0


set -e

function runTests {
  echo "==> Checking source code against AzureRM contributing guidelines..."
	azurerm-linter -use-git-repo=false ./internal/services/...
}

function main {
  runTests
}

main
