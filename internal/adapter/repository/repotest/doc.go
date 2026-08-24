// Package repotest holds the behaviour every user.Repository must exhibit.
//
// It is a package rather than a file so that the in-memory and Postgres
// adapters run the identical assertions. Two implementations behind one port
// that disagree about which error a bad call produces are two different ports
// wearing the same name -- and internal/api/http/helpers maps those errors to
// status codes exactly once, so only one of the two can be right.
//
// It lives under internal/adapter/repository rather than in either adapter
// because neither adapter owns the contract; the domain does.
package repotest
