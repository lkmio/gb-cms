package main

import "sync"

var (
	SinkManager = NewSinkManager()
)

type sinkManager struct {
	lock          sync.RWMutex
	streamSinks   map[StreamID]map[string]*Sink // 推流id->sinks(sinkId->sink)
	callIds       map[string]*Sink              // callId->sink
	sinkStreamIds map[StreamID]*Sink            // sinkStreamId->sink, 关联广播sink
}

func (s *sinkManager) Add(sink *Sink) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	streamSinks, ok := s.streamSinks[sink.Stream]
	if !ok {
		streamSinks = make(map[string]*Sink)
		s.streamSinks[sink.Stream] = streamSinks
	}

	if sink.Dialog == nil {
		return false
	}

	callId, _ := sink.Dialog.CallID()
	id := callId.Value()
	if _, ok := s.callIds[id]; ok {
		return false
	} else if _, ok := streamSinks[sink.ID]; ok {
		return false
	}

	s.callIds[id] = sink
	s.streamSinks[sink.Stream][sink.ID] = sink
	return true
}

func (s *sinkManager) AddWithSinkStreamId(sink *Sink) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.sinkStreamIds[sink.SinkStream]; ok {
		return false
	}
	s.sinkStreamIds[sink.SinkStream] = sink
	return true
}

func (s *sinkManager) Remove(stream StreamID, sinkID string) *Sink {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.streamSinks[stream]; !ok {
		return nil
	}

	sink, ok := s.streamSinks[stream][sinkID]
	if !ok {
		return nil
	}

	s.removeSink(sink)
	return sink
}

func (s *sinkManager) RemoveWithCallId(callId string) *Sink {
	s.lock.Lock()
	defer s.lock.Unlock()

	if sink, ok := s.callIds[callId]; ok {
		s.removeSink(sink)
		return sink
	}

	return nil
}

func (s *sinkManager) removeSink(sink *Sink) {
	delete(s.streamSinks[sink.Stream], sink.ID)

	if sink.Dialog != nil {
		callID, _ := sink.Dialog.CallID()
		delete(s.callIds, callID.Value())
	}

	if sink.SinkStream != "" {
		delete(s.sinkStreamIds, sink.SinkStream)
	}
}

func (s *sinkManager) RemoveWithSinkStreamId(sinkStreamId StreamID) *Sink {
	s.lock.Lock()
	defer s.lock.Unlock()
	if sink, ok := s.sinkStreamIds[sinkStreamId]; ok {
		s.removeSink(sink)
		return sink
	}

	return nil
}

func (s *sinkManager) Find(stream StreamID, sinkID string) *Sink {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if _, ok := s.streamSinks[stream]; !ok {
		return nil
	}

	sink, ok := s.streamSinks[stream][sinkID]
	if !ok {
		return nil
	}

	return sink
}

func (s *sinkManager) FindWithCallId(callId string) *Sink {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if sink, ok := s.callIds[callId]; ok {
		return sink
	}

	return nil
}

func (s *sinkManager) FindWithSinkStreamId(sinkStreamId StreamID) *Sink {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if sink, ok := s.sinkStreamIds[sinkStreamId]; ok {
		return sink
	}

	return nil
}

func (s *sinkManager) PopSinks(stream StreamID) []*Sink {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.streamSinks[stream]; !ok {
		return nil
	}

	var sinkList []*Sink
	for _, sink := range s.streamSinks[stream] {
		sinkList = append(sinkList, sink)
	}

	for _, sink := range sinkList {
		s.removeSink(sink)
	}

	delete(s.streamSinks, stream)
	return sinkList
}

func AddForwardSink(StreamID StreamID, sink *Sink) bool {
	if !SinkManager.Add(sink) {
		Sugar.Errorf("转发Sink添加失败, StreamID: %s SinkID: %s", StreamID, sink.ID)
		return false
	}

	if DB != nil {
		err := DB.SaveForwardSink(StreamID, sink)
		if err != nil {
			Sugar.Errorf("转发Sink保存到数据库失败, err: %s", err.Error())
		}
	}

	return true
}

func RemoveForwardSink(StreamID StreamID, sinkID string) *Sink {
	sink := SinkManager.Remove(StreamID, sinkID)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func RemoveForwardSinkWithCallId(callId string) *Sink {
	sink := SinkManager.RemoveWithCallId(callId)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func RemoveForwardSinkWithSinkStreamId(sinkStreamId StreamID) *Sink {
	sink := SinkManager.RemoveWithSinkStreamId(sinkStreamId)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func releaseSink(sink *Sink) {
	if DB != nil {
		err := DB.DeleteForwardSink(sink.Stream, sink.ID)
		if err != nil {
			Sugar.Errorf("删除转发Sink失败, err: %s", err.Error())
		}
	}

	// 减少拉流计数
	if stream := StreamManager.Find(sink.Stream); stream != nil {
		stream.DecreaseSinkCount()
	}
}

func closeSink(sink *Sink, bye, ms bool) {
	releaseSink(sink)

	var callId string
	if sink.Dialog != nil {
		callId_, _ := sink.Dialog.CallID()
		callId = callId_.Value()
	}

	platform := PlatformManager.FindPlatform(sink.ServerID)
	if platform != nil {
		platform.CloseStream(callId, bye, ms)
	} else {
		sink.Close(bye, ms)
	}
}

func CloseStreamSinks(StreamID StreamID, bye, ms bool) []*Sink {
	sinks := SinkManager.PopSinks(StreamID)

	for _, sink := range sinks {
		closeSink(sink, bye, ms)
	}

	// 查询数据库中的残余sink
	if DB != nil {
		// 恢复级联转发sink
		forwardSinks, _ := DB.QueryForwardSinks(StreamID)
		for _, sink := range forwardSinks {
			closeSink(sink, bye, ms)
		}
	}

	// 删除整个转发流
	if DB != nil {
		err := DB.Del(ForwardSinksKey(string(StreamID)))
		if err != nil {
			Sugar.Errorf("删除转发Sink失败, err: %s", err.Error())
		}
	}

	return sinks
}

func FindSink(StreamID StreamID, sinkID string) *Sink {
	return SinkManager.Find(StreamID, sinkID)
}

func NewSinkManager() *sinkManager {
	return &sinkManager{
		streamSinks:   make(map[StreamID]map[string]*Sink),
		callIds:       make(map[string]*Sink),
		sinkStreamIds: make(map[StreamID]*Sink),
	}
}
