package main

func AddForwardSink(StreamID StreamID, sink *Sink) bool {
	if err := SinkDao.SaveForwardSink(StreamID, sink); err != nil {
		Sugar.Errorf("保存sink到数据库失败, stream: %s sink: %s err: %s", StreamID, sink.SinkID, err.Error())
		return false
	}

	return true
}

func RemoveForwardSink(StreamID StreamID, sinkID string) *Sink {
	sink, _ := SinkDao.DeleteForwardSink(StreamID, sinkID)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func RemoveForwardSinkWithCallId(callId string) *Sink {
	sink, _ := SinkDao.DeleteForwardSinkByCallID(callId)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func RemoveForwardSinkWithSinkStreamID(sinkStreamId StreamID) *Sink {
	sink, _ := SinkDao.DeleteForwardSinkBySinkStreamID(sinkStreamId)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func releaseSink(sink *Sink) {
	// 减少拉流计数
	//if stream := StreamManager.Find(sink.StreamID); stream != nil {
	//	stream.DecreaseSinkCount()
	//}
}

func closeSink(sink *Sink, bye, ms bool) {
	releaseSink(sink)

	var callId string
	if sink.Dialog != nil {
		callId_, _ := sink.Dialog.CallID()
		callId = callId_.Value()
	}

	platform := PlatformManager.Find(sink.ServerAddr)
	if platform != nil {
		platform.CloseStream(callId, bye, ms)
	} else {
		sink.Close(bye, ms)
	}
}

func CloseStreamSinks(StreamID StreamID, bye, ms bool) []*Sink {
	sinks, _ := SinkDao.DeleteForwardSinksByStreamID(StreamID)
	for _, sink := range sinks {
		closeSink(sink, bye, ms)
	}

	return sinks
}
