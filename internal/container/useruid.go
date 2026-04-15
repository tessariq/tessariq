package container

// RepairImage is the pinned helper image used by host-side permission and
// ownership repair flows before and after container execution.
const RepairImage = "alpine@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659"

// TessariqUser is the named non-root user Tessariq launches in compatible
// runtime images.
const TessariqUser = "tessariq"
