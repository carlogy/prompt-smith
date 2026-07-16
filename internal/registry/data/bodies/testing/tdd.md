Work in a strict red-green-refactor loop, one behavior at a time end-to-end
rather than writing every test up front. Confirm the interface and the
prioritized list of behaviors before starting. For each behavior: write a
failing test that describes what the system does through its public interface
(not how it does it internally), then write the minimal code needed to pass
that one test, then move to the next behavior. Only refactor once green, never
while a test is red. Mock only at true system boundaries — external APIs,
time, randomness — never your own internal modules, since a good test should
survive an internal refactor and only break when real behavior changes.
