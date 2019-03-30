// Package ldbstore implements CFR storage components that keep data
// on disk in a LevelDB database, rather than in memory.
//
// These implementations are substantially slower than the corresponding in-memory
// components but can scale to games that do not fit in memory.
package ldbstore
