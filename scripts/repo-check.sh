#!/bin/bash

# Exit on error
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check 1: Look for exec.Command in tmux server tests

# Find all test files in servers/tmux
test_files=$(find servers/tmux -name "*_test.go" -type f 2>/dev/null || true)

if [ -z "$test_files" ]; then
    # No test files, but that's OK
    test_files=""
fi

# Track if we found any violations
found_violations=false
violation_files=()

# Check each test file for exec.Command
for file in $test_files; do
    # Look for exec.Command usage
    if grep -n "exec\.Command" "$file" >/dev/null 2>&1; then
        found_violations=true
        violation_files+=("$file")
    fi
done

# Report results
if [ "$found_violations" = true ]; then
    echo "Running repository checks..."
    echo
    echo "Checking for exec.Command usage in tmux server tests..."
    echo -e "${RED}✗ Found direct exec.Command usage in test files:${NC}"
    echo
    
    for file in "${violation_files[@]}"; do
        echo -e "${RED}  $file:${NC}"
        # Show the lines with exec.Command
        grep -n "exec\.Command" "$file" | while IFS= read -r line; do
            echo "    $line"
        done
        echo
    done
    
    echo -e "${RED}Tests should use helper functions like createUniqueSession() instead of exec.Command()${NC}"
    echo -e "${RED}Helper functions provide proper cleanup and session management${NC}"
    exit 1
fi

# Check 2: Look for Skip statements in tests

# Find all test files
all_test_files=$(find . -name "*_test.go" -type f 2>/dev/null || true)

if [ -n "$all_test_files" ]; then
    # Track if we found any skip statements
    found_skips=false
    skip_files=()
    
    # Check each test file for t.Skip, t.Skipf, t.SkipNow
    for file in $all_test_files; do
        # Look for various forms of Skip
        if grep -E "t\.(Skip|Skipf|SkipNow)" "$file" >/dev/null 2>&1; then
            found_skips=true
            skip_files+=("$file")
        fi
    done
    
    # Report results
    if [ "$found_skips" = true ]; then
        echo "Running repository checks..."
        echo
        echo "Checking for Skip statements in test files..."
        echo -e "${RED}✗ Found Skip statements in test files:${NC}"
        echo
        
        for file in "${skip_files[@]}"; do
            echo -e "${RED}  $file:${NC}"
            # Show the lines with Skip
            grep -nE "t\.(Skip|Skipf|SkipNow)" "$file" | while IFS= read -r line; do
                echo "    $line"
            done
            echo
        done
        
        echo -e "${RED}Tests should fail rather than skip when requirements aren't met${NC}"
        echo -e "${RED}This ensures CI/test environments are properly configured${NC}"
        exit 1
    fi
fi

# If we got here, all checks passed
echo "repo-check.sh: OK"