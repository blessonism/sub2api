package service

import "context"

type channelMonitorGroupReader interface {
	GetByID(context.Context, int64) (*Group, error)
}

func (s *ChannelMonitorService) SetGroupReader(reader channelMonitorGroupReader) {
	s.groupReader = reader
}

func (s *ChannelMonitorService) validateLinkedGroup(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	if *id <= 0 || s.groupReader == nil {
		return ErrGroupNotFound
	}
	group, err := s.groupReader.GetByID(ctx, *id)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	return nil
}
