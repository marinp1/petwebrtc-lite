package main

import "embed"

//go:embed public/*
var content embed.FS
