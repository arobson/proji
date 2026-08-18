package git

import "errors"

// ErrNothingToCommit means there were no staged changes to commit.
var ErrNothingToCommit = errors.New("nothing to commit")
