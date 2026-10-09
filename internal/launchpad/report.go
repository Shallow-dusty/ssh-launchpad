package launchpad

// failureReason preserves coarse process exit codes while keeping distinct
// failure causes machine-readable. Specific callers can set a finer reason
// (for example mutation_busy versus confirmation_required) before finishing.
func failureReason(stage Stage, code int) FailureReason {
	switch code {
	case ExitInvalidProfile:
		return ReasonInvalidProfile
	case ExitVerificationFailed:
		return ReasonVerificationFailed
	case ExitNeedsElevation:
		return ReasonElevationRequired
	case ExitConfirmationRequired:
		return ReasonConfirmationRequired
	case ExitSelfCutBlocked:
		return ReasonSelfCutBlocked
	case ExitDownloadFailure:
		return ReasonDownloadFailed
	case ExitUnsupported:
		return ReasonUnsupported
	case ExitPartialFailure:
		if stage == StageRollback {
			return ReasonRollbackFailed
		}
		return ReasonExecutionFailed
	default:
		return ""
	}
}
