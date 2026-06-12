package engine

// testSession is shared by tests that only read resolved facts, mirroring the
// process-wide memoization the package had before sessions; tests that need a
// cold cache create their own Session.
var testSession = NewSession()
