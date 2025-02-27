// SPDX-License-Identifier: Apache-2.0
// Copyright 2020-2021 Authors of Cilium

package check

import (
	"fmt"
	"io"
	"runtime"
	"time"
)

const (
	debug = "🐛"
	info  = "ℹ️ "
	warn  = "⚠️ "
	fail  = "❌"
	fatal = "🟥"

	testPrefix = "  "
)

//
// Output methods on the global ConnectivityTest context.
// These methods never buffer any lines and are sent directly to the
// user-specified writer.
//

// Header prints a newline followed by a formatted message.
func (ct *ConnectivityTest) Header(a ...interface{}) {
	fmt.Fprintln(ct.params.Writer, "")
	fmt.Fprintln(ct.params.Writer, a...)
}

// Headerf prints a newline followed by a formatted message.
func (ct *ConnectivityTest) Headerf(format string, a ...interface{}) {
	fmt.Fprintf(ct.params.Writer, "\n"+format+"\n", a...)
}

// Log logs a message.
func (ct *ConnectivityTest) Log(a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprintln(ct.params.Writer, a...)
}

// Logf logs a formatted message.
func (ct *ConnectivityTest) Logf(format string, a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprintf(ct.params.Writer, format+"\n", a...)
}

// Debug logs a debug message.
func (ct *ConnectivityTest) Debug(a ...interface{}) {
	if ct.debug() {
		if ct.timestamp() {
			fmt.Fprint(ct.params.Writer, timestamp())
		}
		fmt.Fprint(ct.params.Writer, debug+" ")
		fmt.Fprintln(ct.params.Writer, a...)
	}
}

// Debugf logs a formatted debug message.
func (ct *ConnectivityTest) Debugf(format string, a ...interface{}) {
	if ct.debug() {
		if ct.timestamp() {
			fmt.Fprint(ct.params.Writer, timestamp())
		}
		fmt.Fprint(ct.params.Writer, debug+" ")
		fmt.Fprintf(ct.params.Writer, format+"\n", a...)
	}
}

// Info logs an informational message.
func (ct *ConnectivityTest) Info(a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, info+" ")
	fmt.Fprintln(ct.params.Writer, a...)
}

// Infof logs a formatted informational message.
func (ct *ConnectivityTest) Infof(format string, a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, info+" ")
	fmt.Fprintf(ct.params.Writer, format+"\n", a...)
}

// Warn logs a warning message.
func (ct *ConnectivityTest) Warn(a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, warn+" ")
	fmt.Fprintln(ct.params.Writer, a...)
}

// Warnf logs a formatted warning message.
func (ct *ConnectivityTest) Warnf(format string, a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, warn+" ")
	fmt.Fprintf(ct.params.Writer, format+"\n", a...)
}

// Fail logs a failure message.
func (ct *ConnectivityTest) Fail(a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, fail+" ")
	fmt.Fprintln(ct.params.Writer, a...)
}

// Failf logs a formatted failure message.
func (ct *ConnectivityTest) Failf(format string, a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, fail+" ")
	fmt.Fprintf(ct.params.Writer, format+"\n", a...)
}

// Fatal logs an error.
func (ct *ConnectivityTest) Fatal(a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, fatal+" ")
	fmt.Fprintln(ct.params.Writer, a...)
}

// Fatalf logs a formatted error.
func (ct *ConnectivityTest) Fatalf(format string, a ...interface{}) {
	if ct.timestamp() {
		fmt.Fprint(ct.params.Writer, timestamp())
	}
	fmt.Fprint(ct.params.Writer, fatal+" ")
	fmt.Fprintf(ct.params.Writer, format+"\n", a...)
}

//
// Output methods on an individual test scope.
// Some of these methods will buffer content until a test is marked as failed.
// Test code should never call the output methods of ConnectivityTest, and
// should always call the methods implemented on Test.
//

// progress outputs an unbuffered progress indicator if logging is buffered.
func (t *Test) progress() {
	t.logMu.RLock()
	defer t.logMu.RUnlock()

	// Skip progress indicator if logging is not buffered.
	if t.logBuf == nil {
		return
	}

	fmt.Fprint(t.ctx.params.Writer, ".")
}

// log takes out a read lock and logs a message to the Test's internal buffer.
// If the internal log buffer is nil, write to user-specified writer instead.
// Prefix is an optional prefix to the message.
func (t *Test) log(prefix string, a ...interface{}) {
	t.logMu.RLock()
	defer t.logMu.RUnlock()

	b := t.logBuf
	if b == nil {
		b = t.ctx.params.Writer
	}

	if t.ctx.timestamp() {
		fmt.Fprint(b, timestamp())
	}

	// Test-level output is indented.
	fmt.Fprint(b, testPrefix)

	// Output the prefix specified by the caller.
	if prefix != "" {
		fmt.Fprint(b, prefix+" ")
	}

	fmt.Fprintln(b, a...)
}

// logf takes out a read lock and logs a formatted message to the Test's
// internal buffer. If the internal log buffer is nil, write to user-specified
// writer instead.
func (t *Test) logf(format string, a ...interface{}) {
	t.logMu.RLock()
	defer t.logMu.RUnlock()

	b := t.logBuf
	if b == nil {
		b = t.ctx.params.Writer
	}

	if t.ctx.timestamp() {
		fmt.Fprint(b, timestamp())
	}

	fmt.Fprintf(b, testPrefix+format+"\n", a...)
}

func (t *Test) flush() {
	// Prevent any other messages from being written to the Test buffer.
	t.logMu.Lock()
	defer t.logMu.Unlock()

	// Nil buffer means we're already sending to user-specified writer.
	if t.logBuf == nil {
		return
	}

	// Terminate progress so far.
	fmt.Fprintln(t.ctx.params.Writer)

	// Flush internal buffer to user-specified writer.
	if _, err := io.Copy(t.ctx.params.Writer, t.logBuf); err != nil {
		panic(err)
	}

	// Assign a nil buffer so future writes go to user-specified writer.
	t.logBuf = nil
}

// Headerf prints a formatted, indented header inside the test log scope.
// Headers are not internally buffered.
func (t *Test) Headerf(format string, a ...interface{}) {
	t.ctx.Headerf(testPrefix+format, a...)
}

// Log logs a message.
func (t *Test) Log(a ...interface{}) {
	t.log("", a...)
}

// Logf logs a formatted message.
func (t *Test) Logf(format string, a ...interface{}) {
	t.logf(format, a...)
}

// Debug logs a debug message.
func (t *Test) Debug(a ...interface{}) {
	if t.ctx.debug() {
		t.log(debug, a...)
	}
}

// Debugf logs a formatted debug message.
func (t *Test) Debugf(format string, a ...interface{}) {
	if t.ctx.debug() {
		t.logf(debug+" "+format, a...)
	}
}

// Info logs an informational message.
func (t *Test) Info(a ...interface{}) {
	t.log(info, a...)
}

// Infof logs a formatted informational message.
func (t *Test) Infof(format string, a ...interface{}) {
	t.logf(info+" "+format, a...)
}

// Fail marks the Test as failed and logs a failure message.
//
// Flushes the Test's internal log buffer. Any further logs against the Test
// will go directly to the user-specified writer.
func (t *Test) Fail(a ...interface{}) {
	t.failed = true
	t.flush()
	t.log(fail, a...)
}

// Failf marks the Test as failed and logs a formatted failure message.
//
// Flushes the Test's internal log buffer. Any further logs against the Test
// will go directly to the user-specified writer.
func (t *Test) Failf(format string, a ...interface{}) {
	t.failed = true
	t.flush()
	t.logf(fail+" "+format, a...)
}

// Fatal marks the test as failed, logs an error and exits the
// calling goroutine.
func (t *Test) Fatal(a ...interface{}) {
	t.failed = true
	t.flush()
	t.log(fatal, a...)
	runtime.Goexit()
}

// Fatalf marks the test as failed, logs a formatted error and exits the
// calling goroutine.
func (t *Test) Fatalf(format string, a ...interface{}) {
	t.failed = true
	t.flush()
	t.logf(fatal+" "+format, a...)
	runtime.Goexit()
}

//
// Output methods on an Action scope.
//

// Log logs a message.
func (a *Action) Log(s ...interface{}) {
	a.test.Log(s...)
}

// Logf logs a formatted message.
func (a *Action) Logf(format string, s ...interface{}) {
	a.test.Logf(format, s...)
}

// Debug logs a debug message.
func (a *Action) Debug(s ...interface{}) {
	if a.test.ctx.debug() {
		a.test.Debug(s...)
	}
}

// Debugf logs a formatted debug message.
func (a *Action) Debugf(format string, s ...interface{}) {
	if a.test.ctx.debug() {
		a.test.Debugf(format, s...)
	}
}

// Info logs a debug message.
func (a *Action) Info(s ...interface{}) {
	a.test.Info(s...)
}

// Infof logs a formatted debug message.
func (a *Action) Infof(format string, s ...interface{}) {
	a.test.Infof(format, s...)
}

// Fail must be called when the Action is unsuccessful.
func (a *Action) Fail(s ...interface{}) {
	a.fail()
	a.test.Fail(s...)
}

// Failf must be called when the Action is unsuccessful.
func (a *Action) Failf(format string, s ...interface{}) {
	a.fail()
	a.test.Failf(format, s...)
}

// Fatal must be called when an irrecoverable error was encountered during the Action.
func (a *Action) Fatal(s ...interface{}) {
	a.fail()
	a.test.Fatal(s...)
}

// Fatalf must be called when an irrecoverable error was encountered during the Action.
func (a *Action) Fatalf(format string, s ...interface{}) {
	a.fail()
	a.test.Fatalf(format, s...)
}

func timestamp() string {
	return fmt.Sprintf("[%s] ", time.Now().Format(time.RFC3339))
}
