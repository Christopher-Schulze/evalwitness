package protocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

type AdapterCommand struct {
	Path             string
	Arguments        []string
	WorkingDirectory string
}

type Evaluator interface {
	Descriptor() EvaluatorDescriptor
	Evaluate(AuditCase) InvocationResult
}

type adapterState int

const (
	adapterAwaitHello adapterState = iota
	adapterNegotiated
	adapterDescribed
	adapterRunning
	adapterEnded
)

func ServeReferenceAdapter(input io.Reader, output io.Writer) error {
	return ServeAdapter(input, output, ReferenceEvaluator{})
}

func ServeAdapter(input io.Reader, output io.Writer, evaluator Evaluator) error {
	if evaluator == nil {
		return errors.New("protocol adapter evaluator is required")
	}
	descriptor := evaluator.Descriptor()
	if err := ValidateDescriptor(descriptor); err != nil {
		return fmt.Errorf("validate protocol adapter descriptor: %w", err)
	}
	limits := descriptor.Limits
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), limits.MaxMessageBytes)
	state := adapterAwaitHello
	selectedVersion := ""
	evaluated := 0
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var message AdapterMessage
		if err := DecodeStrict(line, &message); err != nil {
			return fmt.Errorf("decode adapter message: %w", err)
		}
		if err := validateAdapterMessage(message, selectedVersion); err != nil {
			if writeErr := writeAdapterError(output, message, selectedVersion, "protocol.message.invalid", err.Error()); writeErr != nil {
				return writeErr
			}
			return err
		}
		switch message.MessageType {
		case MessageHello:
			if state != adapterAwaitHello {
				return stateError(output, message, selectedVersion, "hello is only valid as the first message")
			}
			var hello Hello
			if err := DecodeStrict(message.Payload, &hello); err != nil {
				return stateError(output, message, selectedVersion, err.Error())
			}
			selectedVersion = negotiateVersion(hello.SupportedVersions, descriptor.ProtocolVersions)
			if selectedVersion == "" || hello.CanonicalPolicy != CanonicalPolicy {
				return stateError(output, message, selectedVersion, "no common version or canonical policy")
			}
			state = adapterNegotiated
			if err := writeAdapterReply(output, message, selectedVersion, MessageHelloAck, HelloAck{SelectedVersion: selectedVersion, CanonicalPolicy: CanonicalPolicy}); err != nil {
				return err
			}
		case MessageDescribe:
			if state != adapterNegotiated {
				return stateError(output, message, selectedVersion, "describe requires a negotiated session")
			}
			state = adapterDescribed
			if err := writeAdapterReply(output, message, selectedVersion, MessageDescriptor, descriptor); err != nil {
				return err
			}
		case MessageBeginRun:
			if state != adapterDescribed {
				return stateError(output, message, selectedVersion, "begin_run requires descriptor exchange")
			}
			var boundary RunBoundary
			if err := DecodeStrict(message.Payload, &boundary); err != nil || boundary.RunID == "" ||
				boundary.CaseCount < 0 || boundary.CaseCount > limits.MaxCasesPerRun || !validDigest(boundary.CorpusDigest) {
				return stateError(output, message, selectedVersion, "begin_run boundary is invalid")
			}
			state = adapterRunning
			if err := writeAdapterReply(output, message, selectedVersion, MessageRunStarted, boundary); err != nil {
				return err
			}
		case MessageEvaluate:
			if state != adapterRunning {
				return stateError(output, message, selectedVersion, "evaluate requires an active run")
			}
			result := evaluateAdapterPayload(evaluator, message.Payload, limits)
			evaluated++
			if err := writeAdapterReply(output, message, selectedVersion, MessageEvaluationResult, result); err != nil {
				return err
			}
		case MessageCancel:
			if state != adapterRunning {
				return stateError(output, message, selectedVersion, "cancel requires an active run")
			}
			if err := writeAdapterReply(output, message, selectedVersion, MessageCancelled, ProtocolError{Code: "protocol.cancelled", Message: "no active invocation remained after the sequential boundary", Retryable: false}); err != nil {
				return err
			}
		case MessageEndRun:
			if state != adapterRunning {
				return stateError(output, message, selectedVersion, "end_run requires an active run")
			}
			var boundary RunBoundary
			if err := DecodeStrict(message.Payload, &boundary); err != nil || boundary.CaseCount != evaluated {
				return stateError(output, message, selectedVersion, "end_run case count is invalid")
			}
			state = adapterEnded
			if err := writeAdapterReply(output, message, selectedVersion, MessageRunResult, boundary); err != nil {
				return err
			}
		default:
			return stateError(output, message, selectedVersion, "host sent a response-only or unknown message type")
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read adapter input: %w", err)
	}
	if state != adapterEnded {
		return errors.New("adapter input ended before end_run")
	}
	return nil
}

func RunAdapterCorpus(ctx context.Context, command AdapterCommand, corpus NormativeCorpus) (AuditRun, error) {
	if command.Path == "" {
		return AuditRun{}, errors.New("adapter executable path is empty")
	}
	cmd := exec.CommandContext(ctx, command.Path, command.Arguments...)
	cmd.Dir = command.WorkingDirectory
	cmd.Env = []string{"EVALWITNESS_PROTOCOL_OFFLINE=1"}
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return AuditRun{}, fmt.Errorf("open adapter stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return AuditRun{}, fmt.Errorf("open adapter stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return AuditRun{}, fmt.Errorf("start adapter: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), DefaultLimits().MaxMessageBytes)
	client := adapterClient{input: scanner, output: stdin, version: CurrentVersion}
	descriptor, err := client.initialize(corpus)
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return AuditRun{}, err
	}
	results := make([]CaseResult, 0, len(corpus.Vectors))
	findings := make([]AuditFinding, 0)
	for _, vector := range corpus.Vectors {
		caseResult, caseFindings, err := client.evaluate(vector, descriptor)
		if err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return AuditRun{}, err
		}
		results = append(results, caseResult)
		findings = append(findings, caseFindings...)
	}
	if err := client.end(corpus, len(corpus.Vectors)); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return AuditRun{}, err
	}
	if err := stdin.Close(); err != nil {
		return AuditRun{}, fmt.Errorf("close adapter stdin: %w", err)
	}
	if scanner.Scan() {
		_ = cmd.Wait()
		return AuditRun{}, errors.New("adapter wrote protocol contamination after run_result")
	}
	if err := scanner.Err(); err != nil {
		return AuditRun{}, fmt.Errorf("read final adapter output: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return AuditRun{}, fmt.Errorf("adapter exited unsuccessfully: %w", err)
	}
	run := AuditRun{
		SchemaVersion: RunSchema, ProtocolVersion: client.version, RunID: "run-" + corpus.Digest[:16],
		Evaluator: descriptor, CorpusDigest: corpus.Digest, RequestCorpusDigest: corpus.RequestCorpusDigest,
		SchemaArtifactDigest: corpus.SchemaArtifactDigest,
		Offline:              true, Results: results, Matrix: BuildCapabilityMatrix(results), Findings: findings,
	}
	material := run
	digest, err := Digest(material)
	if err != nil {
		return AuditRun{}, err
	}
	run.RunDigest = digest
	return run, ValidateAuditRun(run)
}

type adapterClient struct {
	input   *bufio.Scanner
	output  io.Writer
	version string
	nextID  int
	runID   string
}

func (client *adapterClient) initialize(corpus NormativeCorpus) (EvaluatorDescriptor, error) {
	hello := Hello{SupportedVersions: SupportedVersions(), CanonicalPolicy: CanonicalPolicy}
	var ack HelloAck
	if err := client.exchange(MessageHello, MessageHelloAck, "", hello, &ack); err != nil {
		return EvaluatorDescriptor{}, err
	}
	if !supportedVersion(ack.SelectedVersion) || ack.CanonicalPolicy != CanonicalPolicy {
		return EvaluatorDescriptor{}, errors.New("adapter selected an unsupported protocol or canonical policy")
	}
	client.version = ack.SelectedVersion
	var descriptor EvaluatorDescriptor
	if err := client.exchange(MessageDescribe, MessageDescriptor, "", struct{}{}, &descriptor); err != nil {
		return EvaluatorDescriptor{}, err
	}
	if err := ValidateDescriptor(descriptor); err != nil {
		return EvaluatorDescriptor{}, err
	}
	client.runID = "run-" + corpus.Digest[:16]
	boundary := RunBoundary{RunID: client.runID, CaseCount: len(corpus.Vectors), CorpusDigest: corpus.Digest}
	var started RunBoundary
	if err := client.exchange(MessageBeginRun, MessageRunStarted, client.runID, boundary, &started); err != nil {
		return EvaluatorDescriptor{}, err
	}
	if started != boundary {
		return EvaluatorDescriptor{}, errors.New("adapter changed the begin_run boundary")
	}
	return descriptor, nil
}

func (client *adapterClient) evaluate(vector CaseVector, descriptor EvaluatorDescriptor) (CaseResult, []AuditFinding, error) {
	caseResult := CaseResult{CaseID: vector.CaseID, Level: vector.Level, Kind: vector.Kind, FindingCodes: []string{}}
	if len(vector.Raw) > descriptor.Limits.MaxCaseBytes {
		caseResult.Outcome = OutcomeFailed
		caseResult.Reason = "case exceeds adapter descriptor max_case_bytes"
		return caseResult, nil, nil
	}
	if !capabilitiesAvailable(vector, descriptor) {
		caseResult.Outcome = OutcomeUnsupported
		caseResult.Reason = "adapter descriptor does not declare the required capability"
		return caseResult, nil, nil
	}
	var result InvocationResult
	if err := client.exchangeRaw(MessageEvaluate, MessageEvaluationResult, client.runID, vector.Raw, &result); err != nil {
		return CaseResult{}, nil, err
	}
	invocationID := invocationIDFromRaw(vector.Raw)
	if err := ValidateInvocationResult(result, invocationID); err != nil {
		caseResult.Outcome = OutcomeFailed
		caseResult.Reason = err.Error()
		return caseResult, result.Findings, nil
	}
	caseResult.ResultDigest = result.EvidenceDigest
	caseResult.ObservedDigest = result.ObservedDigest
	for _, finding := range result.Findings {
		caseResult.FindingCodes = append(caseResult.FindingCodes, finding.Code)
	}
	sort.Strings(caseResult.FindingCodes)
	matched, reason := resultMatchesExpected(vector.Expected, result)
	if matched {
		caseResult.Outcome = OutcomePassed
	} else {
		caseResult.Outcome = OutcomeFailed
	}
	caseResult.Reason = reason
	return caseResult, result.Findings, nil
}

func (client *adapterClient) end(corpus NormativeCorpus, count int) error {
	boundary := RunBoundary{RunID: client.runID, CaseCount: count, CorpusDigest: corpus.Digest}
	var ended RunBoundary
	if err := client.exchange(MessageEndRun, MessageRunResult, client.runID, boundary, &ended); err != nil {
		return err
	}
	if ended != boundary {
		return errors.New("adapter changed the end_run boundary")
	}
	return nil
}

func (client *adapterClient) exchange(messageType, expected MessageType, runID string, payload any, destination any) error {
	raw, err := CanonicalMarshal(payload)
	if err != nil {
		return err
	}
	return client.exchangeRaw(messageType, expected, runID, raw, destination)
}

func (client *adapterClient) exchangeRaw(messageType, expected MessageType, runID string, payload json.RawMessage, destination any) error {
	client.nextID++
	messageID := fmt.Sprintf("host.%06d", client.nextID)
	message := AdapterMessage{
		SchemaVersion: MessageSchema, ProtocolVersion: client.version, MessageType: messageType,
		MessageID: messageID, RunID: runID, Payload: append([]byte(nil), payload...),
	}
	if err := writeMessage(client.output, message); err != nil {
		return err
	}
	if !client.input.Scan() {
		if err := client.input.Err(); err != nil {
			return fmt.Errorf("read adapter reply: %w", err)
		}
		return errors.New("adapter closed stdout before replying")
	}
	var reply AdapterMessage
	if err := DecodeStrict(client.input.Bytes(), &reply); err != nil {
		return fmt.Errorf("decode adapter reply: %w", err)
	}
	if reply.MessageType == MessageError {
		var protocolError ProtocolError
		if err := DecodeStrict(reply.Payload, &protocolError); err != nil {
			return err
		}
		return fmt.Errorf("adapter protocol error %s: %s", protocolError.Code, protocolError.Message)
	}
	if reply.MessageType != expected || reply.ReplyTo != messageID || reply.ProtocolVersion != client.version || reply.RunID != runID {
		return errors.New("adapter reply type, correlation, version, or run identity is invalid")
	}
	return DecodeStrict(reply.Payload, destination)
}

func evaluateAdapterPayload(evaluator Evaluator, raw json.RawMessage, limits ResourceLimits) InvocationResult {
	invocationID := invocationIDFromRaw(raw)
	var auditCase AuditCase
	if err := DecodeStrict(raw, &auditCase); err != nil {
		return sealInvocationResult(InvocationResult{
			SchemaVersion: ResultSchema, InvocationID: invocationID, Status: InvocationRejected,
			Findings: []AuditFinding{findingForError(err)}, Extensions: []Extension{},
		})
	}
	if err := validateAuditCaseRequiredFields(raw); err != nil {
		return sealInvocationResult(InvocationResult{
			SchemaVersion: ResultSchema, InvocationID: invocationID, Status: InvocationRejected,
			Findings: []AuditFinding{findingForError(err)}, Extensions: []Extension{},
		})
	}
	if err := ValidateCaseEnvelope(auditCase, limits); err != nil {
		return sealInvocationResult(InvocationResult{
			SchemaVersion: ResultSchema, InvocationID: auditCase.Invocation.InvocationID, Status: InvocationRejected,
			Findings: []AuditFinding{findingForError(err)}, Extensions: []Extension{},
		})
	}
	if len(raw) > auditCase.Invocation.MaxInputBytes || len(raw) > limits.MaxCaseBytes {
		return sealInvocationResult(InvocationResult{
			SchemaVersion: ResultSchema, InvocationID: invocationID, Status: InvocationRejected,
			Findings: []AuditFinding{{
				SchemaVersion: FindingSchema, Code: "protocol.resource.case_bytes", Severity: "error",
				Path: "/invocation/max_input_bytes", Message: "audit case exceeds its declared byte ceiling",
				Invariant: "case bytes must fit both the invocation and adapter ceilings",
			}}, Extensions: []Extension{},
		})
	}
	return evaluator.Evaluate(auditCase)
}

func invocationIDFromRaw(raw []byte) string {
	var partial struct {
		Invocation struct {
			InvocationID string `json:"invocation_id"`
		} `json:"invocation"`
	}
	if err := json.Unmarshal(raw, &partial); err == nil && partial.Invocation.InvocationID != "" {
		return partial.Invocation.InvocationID
	}
	return "invocation.invalid"
}

func capabilitiesAvailable(vector CaseVector, descriptor EvaluatorDescriptor) bool {
	var partial AuditCase
	if err := json.Unmarshal(vector.Raw, &partial); err != nil {
		return true
	}
	for _, required := range partial.RequiredCapabilities {
		if !containsString(descriptor.ExecutionModes, required) {
			return false
		}
	}
	return true
}

func validateAdapterMessage(message AdapterMessage, selectedVersion string) error {
	if message.SchemaVersion != MessageSchema || message.MessageID == "" || len(message.Payload) == 0 {
		return errors.New("adapter message schema, identity, or payload is invalid")
	}
	if message.MessageType == MessageHello {
		if !supportedVersion(message.ProtocolVersion) {
			return errors.New("hello protocol version is unsupported")
		}
		return nil
	}
	if selectedVersion == "" || message.ProtocolVersion != selectedVersion {
		return errors.New("adapter message protocol version does not match negotiation")
	}
	return nil
}

func negotiateVersion(host, adapter []string) string {
	for _, version := range SupportedVersions() {
		if containsString(host, version) && containsString(adapter, version) {
			return version
		}
	}
	return ""
}

func writeAdapterReply(output io.Writer, request AdapterMessage, version string, messageType MessageType, payload any) error {
	raw, err := CanonicalMarshal(payload)
	if err != nil {
		return err
	}
	reply := AdapterMessage{
		SchemaVersion: MessageSchema, ProtocolVersion: version, MessageType: messageType,
		MessageID: "reply." + request.MessageID, ReplyTo: request.MessageID, RunID: request.RunID, Payload: raw,
	}
	return writeMessage(output, reply)
}

func writeAdapterError(output io.Writer, request AdapterMessage, version, code, message string) error {
	if version == "" {
		version = CurrentVersion
	}
	return writeAdapterReply(output, request, version, MessageError, ProtocolError{Code: code, Message: message, Retryable: false})
}

func stateError(output io.Writer, request AdapterMessage, version, message string) error {
	if err := writeAdapterError(output, request, version, "protocol.state.invalid", message); err != nil {
		return err
	}
	return errors.New(message)
}

func writeMessage(output io.Writer, message AdapterMessage) error {
	raw, err := CanonicalMarshal(message)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if _, err := io.Copy(output, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("write adapter message: %w", err)
	}
	return nil
}
