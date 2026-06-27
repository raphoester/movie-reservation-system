#!/bin/bash

echo "proto: removing previously generated files"
find . -iname "*.pb.go" -exec rm {} +

if ! command -v buf &> /dev/null; then
    echo "proto: error: buf is not installed. Please check README."
    exit 1
fi

echo "proto: running buf lint"
if ! buf lint; then
    echo "proto: error: buf lint failed"
    exit 1
fi

echo "proto: running buf"
if ! buf generate; then
    echo "proto: error: buf generate failed"
    exit 1
fi