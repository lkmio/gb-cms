package main

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
)

type RedisUtils struct {
	pool     *redis.Pool
	password string
}

func (utils *RedisUtils) CreateExecutor() (Executor, error) {
	conn := utils.pool.Get()
	if utils.password != "" {
		if _, err := conn.Do("auth", utils.password); err != nil {
			return nil, err
		}
	}

	//if _, err := conn.Do("select", index); err != nil {
	//	return nil, err
	//}

	return &RedisExecutor{
		conn: conn,
	}, nil
}

type Executor interface {
	DB(index int) Executor

	Key(key string) Executor

	Do(commandName string, args ...interface{}) (reply interface{}, err error)

	// Keys 返回所有主键
	Keys() ([]string, error)

	Set(value interface{}) error

	Get() (interface{}, error)

	Exist() (bool, error)

	Del() error

	HSet(k string, v interface{}) error

	HGet(k string) ([]byte, error)

	HGetAll() (map[string][]byte, error)

	HExist(k string) (bool, error)

	HDel(k string) error

	HSan(page, size int) ([][]string, error)

	ZAdd(score interface{}, v interface{}) error

	ZAddWithNotExists(score interface{}, v interface{}) error

	// ZRangeWithDesc 降序查询
	ZRangeWithDesc(page, size int) ([]string, error)

	// ZRangeWithAsc 升序查询
	ZRangeWithAsc(page, size int) ([]string, error)

	// ZRange 返回zset所有元素
	ZRange() ([][2]string, error)

	ZGetScore(member interface{}) (interface{}, error)

	ZDel(member interface{}) error

	ZDelWithScore(score interface{}) error

	// CountZSet 返回zset元素个数
	CountZSet() (int, error)

	SetExpires(expires int) error
}

type RedisExecutor struct {
	conn redis.Conn
	db   int
	key  string
}

func (e *RedisExecutor) DB(index int) Executor {
	e.db = index
	return e
}

func (e *RedisExecutor) Key(key string) Executor {
	e.key = key
	return e
}

func (e *RedisExecutor) Do(commandName string, args ...interface{}) (interface{}, error) {
	if _, err := e.conn.Do("select", e.db); err != nil {
		return nil, err
	}

	return e.conn.Do(commandName, args...)
}

func (e *RedisExecutor) Keys() ([]string, error) {
	data, err := e.Do("keys", "*")
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, key := range data.([]interface{}) {
		keys = append(keys, string(key.([]uint8)))
	}

	return keys, nil
}

func (e *RedisExecutor) SetExpires(expires int) error {
	_, err := e.Do("expire", e.key, expires)
	return err
}

// HSet 设置map元素
func (e *RedisExecutor) HSet(entryK string, entryV interface{}) error {
	_, err := e.Do("hset", e.key, entryK, entryV)
	return err
}

func (e *RedisExecutor) Del() error {
	_, err := e.Do("del", e.key)
	return err
}

func (e *RedisExecutor) HDel(entryK string) error {
	_, err := e.Do("del", e.key, entryK)
	return err
}

func (e *RedisExecutor) Set(value interface{}) error {
	_, err := e.Do("set", e.key, value)
	return err
}

func (e *RedisExecutor) Get() (interface{}, error) {
	return e.Do("get", e.key)
}

func (e *RedisExecutor) Exist() (bool, error) {
	return redis.Bool(e.Do("exist", e.key))
}

func (e *RedisExecutor) HGet(k string) ([]byte, error) {
	data, err := e.Do("hget", e.key, k)
	if err != nil {
		return nil, err
	} else if data == nil {
		return nil, nil
	}

	return data.([]byte), err
}

func (e *RedisExecutor) HGetAll() (map[string][]byte, error) {
	data, err := e.Do("hgetall", e.key)
	if err != nil {
		return nil, err
	} else if data == nil {
		return nil, err
	}

	entries := data.([]interface{})
	n := len(entries) / 2
	result := make(map[string][]byte, n)
	for i := 0; i < n; i++ {
		result[string(entries[i*2].([]uint8))] = entries[i*2+1].([]uint8)
	}

	return result, err
}

func (e *RedisExecutor) HExist(k string) (bool, error) {
	return redis.Bool(e.Do("exist", e.key, k))
}

func (e *RedisExecutor) HSan(page, size int) ([][]string, error) {
	reply, err := e.Do("hscan", e.key, (page-1)*size, "count", size)
	if err != nil {
		return nil, err
	}

	response, _ := reply.([]interface{})
	_ = response[0]
	data := response[1].([]interface{})

	n := len(data) / 2
	var result [][]string
	for i := 0; i < n; i++ {
		pair := make([]string, 2)
		pair[0] = string(data[i*2].([]uint8))
		pair[1] = string(data[i*2+1].([]uint8))
		result = append(result, pair)
	}

	return result, err
}

func (e *RedisExecutor) ZAdd(score interface{}, v interface{}) error {
	_, err := e.Do("zadd", e.key, score, v)
	return err
}

func (e *RedisExecutor) ZAddWithNotExists(score interface{}, v interface{}) error {
	_, err := e.Do("zadd", e.key, "nx", score, v)
	return err
}

func (e *RedisExecutor) ZRangeWithDesc(page, size int) ([]string, error) {
	reply, err := e.Do("zrevrange", e.key, (page-1)*size, (page-1)*size+size-1)
	data := reply.([]interface{})

	var result []string
	for _, v := range data {
		result = append(result, string(v.([]uint8)))
	}

	return result, err
}

func (e *RedisExecutor) ZRangeWithAsc(page, size int) ([]string, error) {
	reply, err := e.Do("zrange", e.key, (page-1)*size, (page-1)*size+size-1)
	data := reply.([]interface{})

	var result []string
	for _, v := range data {
		result = append(result, string(v.([]uint8)))
	}

	return result, err
}

func (e *RedisExecutor) ZDel(member interface{}) error {
	_, err := e.Do("zrem", e.key, member)
	return err
}

func (e *RedisExecutor) ZDelWithScore(score interface{}) error {
	_, err := e.Do("zremrangebyscore", e.key, score, score)
	return err
}

func (e *RedisExecutor) ZRange() ([][2]string, error) {
	reply, err := e.Do("zrange", e.key, 0, -1, "withscores")
	if err != nil {
		return nil, err
	}

	data := reply.([]interface{})
	n := len(data) / 2

	var result [][2]string
	for i := 0; i < n; i++ {
		var pair [2]string
		pair[0] = string(data[i*2].([]uint8))
		pair[1] = string(data[i*2+1].([]uint8))
		result = append(result, pair)
	}

	return result, nil
}

func (e *RedisExecutor) ZGetScore(member interface{}) (interface{}, error) {
	return e.Do("zrank", e.key, member)
}

func (e *RedisExecutor) CountZSet() (int, error) {
	do, err := e.Do("zcard", e.key)
	if err != nil {
		return 0, err
	}

	return int(do.(int64)), err
}

func StartExpiredKeysSubscription(utils *RedisUtils, db int, cb func(db int, key string)) error {
	conn := utils.pool.Get()

	if "" != utils.password {
		if _, err := conn.Do("auth", utils.password); err != nil {
			return err
		}
	}

	if _, err := conn.Do("config", "set", "protected-mode", "no"); err != nil {
		return err
	}

	if _, err := conn.Do("config", "set", "notify-keyspace-events", "AE"); err != nil {
		return err
	}

	redisClient := redis.PubSubConn{Conn: conn}
	pattern := fmt.Sprintf("__keyevent@%d__:expired", db)
	if err := redisClient.PSubscribe(pattern); err != nil {
		return err
	}

	go func() {
		for {
			switch msg := redisClient.Receive().(type) {
			case redis.Message:
				if pattern == msg.Pattern {
					key := string(msg.Data)
					go cb(db, key)
				}
				break
			case redis.Subscription:
				break
			case error:
				break
			}
		}
	}()

	return nil
}

func NewRedisUtils(addr, password string) *RedisUtils {
	return &RedisUtils{
		pool: &redis.Pool{
			MaxIdle:     50,   // 最大空闲连接数
			MaxActive:   0,    // 和数据库的最大连接数，0 表示没有限制
			IdleTimeout: 1000, // 最大空闲时间
			Dial: func() (redis.Conn, error) { // 初始化连接的代码
				return redis.Dial("tcp", addr)
			},
		},

		password: password,
	}
}
