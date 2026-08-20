package process

type processTree interface {
	SoftCancel() error
	GracefulTerminate() error
	HardKill() error
	Close() error
}
