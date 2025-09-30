#!/bin/bash
cd server
go mod tidy
cd ../client
npm ci