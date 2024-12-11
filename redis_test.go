package main

import (
	utils2 "github.com/lkmio/avformat/utils"
	"strconv"
	"testing"
	"time"
)

func TestRedisUtils(t *testing.T) {
	utils := NewRedisUtils("localhost:6379", "")

	executor, err := utils.CreateExecutor()
	if err != nil {
		panic(err)
	}

	executor.DB(1)

	t.Run("map", func(t *testing.T) {
		err = executor.Key("utils").HSet("user", "name")
		if err != nil {
			panic(err)
		}

		get, err2 := executor.HGet("user")
		utils2.Assert(err2 == nil)
		println(get)

		all, err2 := executor.HGetAll()

		utils2.Assert(err2 == nil)
		println(all)

		for i := 0; i < 10000; i++ {
			executor.HSet(strconv.Itoa(i), strconv.Itoa(i))
		}

		executor.HSan(1, 10)

		err = executor.Key("key_expires").Set("name")
		err = executor.SetExpires(10)
	})

	t.Run("zset", func(t *testing.T) {
		_, err = executor.ZRange()

		err = executor.Key("zset").ZAddWithNotExists(float64(time.Now().UnixMilli()), 1)
		utils2.Assert(err == nil)
		err = executor.Key("zset").ZAddWithNotExists(float64(time.Now().UnixMilli()), 1)
		utils2.Assert(err == nil)

		for i := 0; i < 10; i++ {
			if err = executor.Key("zset").ZAdd(float64(i), i); err != nil {
				panic(err)
			}
		}

		score, err := executor.Key("zset").ZGetScore(9)
		utils2.Assert(err != nil)
		println(score)

		score, err = executor.Key("zset").ZGetScore(100)
		utils2.Assert(err != nil)
		println(score)

		_, err = executor.ZRange()
		utils2.Assert(err == nil)

		asc, err2 := executor.ZRangeWithAsc(1, 5)
		if err2 != nil {
			panic(err2)
		}

		println(asc)

		values, err2 := executor.ZRangeWithDesc(1, 5)
		if err2 != nil {
			panic(err2)
		}

		println(values)

		for i := 0; i < 5; i++ {
			executor.ZDel(i)
		}

		set, err := executor.CountZSet()
		println(set)
	})

	err = StartExpiredKeysSubscription(utils, 1, func(db int, key string) {
		println("redis 过期. key: " + key)
	})

	keys, err := executor.Keys()
	utils2.Assert(err == nil)
	println(keys)

	select {}
}
