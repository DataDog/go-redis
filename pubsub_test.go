package redis_test

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/DataDog/go-redis/v8"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PubSub", func() {
	var client *redis.Client

	BeforeEach(func() {
		opt := redisOptions()
		opt.MinIdleConns = 0
		opt.MaxConnAge = 0
		client = redis.NewClient(opt)
		Expect(client.FlushDB().Err()).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.Close()).NotTo(HaveOccurred())
	})

	It("implements Stringer", func() {
		pubsub := client.PSubscribe("mychannel*")
		defer pubsub.Close()

		Expect(pubsub.String()).To(Equal("PubSub(mychannel*)"))
	})

	It("should support pattern matching", func() {
		pubsub := client.PSubscribe("mychannel*")
		defer pubsub.Close()

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Subscription)
			Expect(subscr.Kind).To(Equal("psubscribe"))
			Expect(subscr.Channel).To(Equal("mychannel*"))
			Expect(subscr.Count).To(Equal(1))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err.(net.Error).Timeout()).To(Equal(true))
			Expect(msgi).To(BeNil())
		}

		n, err := client.Publish("mychannel1", "hello").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(int64(1)))

		Expect(pubsub.PUnsubscribe("mychannel*")).NotTo(HaveOccurred())

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Message)
			Expect(subscr.Channel).To(Equal("mychannel1"))
			Expect(subscr.Pattern).To(Equal("mychannel*"))
			Expect(subscr.Payload).To(Equal("hello"))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Subscription)
			Expect(subscr.Kind).To(Equal("punsubscribe"))
			Expect(subscr.Channel).To(Equal("mychannel*"))
			Expect(subscr.Count).To(Equal(0))
		}

		stats := client.PoolStats()
		Expect(stats.Misses).To(Equal(uint32(1)))
	})

	It("should pub/sub channels", func() {
		channels, err := client.PubSubChannels("mychannel*").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(channels).To(BeEmpty())

		pubsub := client.Subscribe("mychannel", "mychannel2")
		defer pubsub.Close()

		channels, err = client.PubSubChannels("mychannel*").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(channels).To(ConsistOf([]string{"mychannel", "mychannel2"}))

		channels, err = client.PubSubChannels("").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(channels).To(BeEmpty())

		channels, err = client.PubSubChannels("*").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(channels)).To(BeNumerically(">=", 2))
	})

	It("should return the numbers of subscribers", func() {
		pubsub := client.Subscribe("mychannel", "mychannel2")
		defer pubsub.Close()

		channels, err := client.PubSubNumSub("mychannel", "mychannel2", "mychannel3").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(channels).To(Equal(map[string]int64{
			"mychannel":  1,
			"mychannel2": 1,
			"mychannel3": 0,
		}))
	})

	It("should return the numbers of subscribers by pattern", func() {
		num, err := client.PubSubNumPat().Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(num).To(Equal(int64(0)))

		pubsub := client.PSubscribe("*")
		defer pubsub.Close()

		num, err = client.PubSubNumPat().Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(num).To(Equal(int64(1)))
	})

	It("should pub/sub", func() {
		pubsub := client.Subscribe("mychannel", "mychannel2")
		defer pubsub.Close()

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Subscription)
			Expect(subscr.Kind).To(Equal("subscribe"))
			Expect(subscr.Channel).To(Equal("mychannel"))
			Expect(subscr.Count).To(Equal(1))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Subscription)
			Expect(subscr.Kind).To(Equal("subscribe"))
			Expect(subscr.Channel).To(Equal("mychannel2"))
			Expect(subscr.Count).To(Equal(2))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err.(net.Error).Timeout()).To(Equal(true))
			Expect(msgi).NotTo(HaveOccurred())
		}

		n, err := client.Publish("mychannel", "hello").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(int64(1)))

		n, err = client.Publish("mychannel2", "hello2").Result()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(int64(1)))

		Expect(pubsub.Unsubscribe("mychannel", "mychannel2")).NotTo(HaveOccurred())

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			msg := msgi.(*redis.Message)
			Expect(msg.Channel).To(Equal("mychannel"))
			Expect(msg.Payload).To(Equal("hello"))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			msg := msgi.(*redis.Message)
			Expect(msg.Channel).To(Equal("mychannel2"))
			Expect(msg.Payload).To(Equal("hello2"))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Subscription)
			Expect(subscr.Kind).To(Equal("unsubscribe"))
			Expect(subscr.Channel).To(Equal("mychannel"))
			Expect(subscr.Count).To(Equal(1))
		}

		{
			msgi, err := pubsub.ReceiveTimeout(time.Second)
			Expect(err).NotTo(HaveOccurred())
			subscr := msgi.(*redis.Subscription)
			Expect(subscr.Kind).To(Equal("unsubscribe"))
			Expect(subscr.Channel).To(Equal("mychannel2"))
			Expect(subscr.Count).To(Equal(0))
		}

		stats := client.PoolStats()
		Expect(stats.Misses).To(Equal(uint32(1)))
	})

	It("should ping/pong", func() {
		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		_, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())

		err = pubsub.Ping("")
		Expect(err).NotTo(HaveOccurred())

		msgi, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())
		pong := msgi.(*redis.Pong)
		Expect(pong.Payload).To(Equal(""))
	})

	It("should ping/pong with payload", func() {
		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		_, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())

		err = pubsub.Ping("hello")
		Expect(err).NotTo(HaveOccurred())

		msgi, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())
		pong := msgi.(*redis.Pong)
		Expect(pong.Payload).To(Equal("hello"))
	})

	It("should multi-ReceiveMessage", func() {
		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		subscr, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(subscr).To(Equal(&redis.Subscription{
			Kind:    "subscribe",
			Channel: "mychannel",
			Count:   1,
		}))

		err = client.Publish("mychannel", "hello").Err()
		Expect(err).NotTo(HaveOccurred())

		err = client.Publish("mychannel", "world").Err()
		Expect(err).NotTo(HaveOccurred())

		msg, err := pubsub.ReceiveMessage()
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Channel).To(Equal("mychannel"))
		Expect(msg.Payload).To(Equal("hello"))

		msg, err = pubsub.ReceiveMessage()
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Channel).To(Equal("mychannel"))
		Expect(msg.Payload).To(Equal("world"))
	})

	It("returns an error when subscribe fails", func() {
		pubsub := client.Subscribe()
		defer pubsub.Close()

		pubsub.SetNetConn(&badConn{
			readErr:  io.EOF,
			writeErr: io.EOF,
		})

		err := pubsub.Subscribe("mychannel")
		Expect(err).To(MatchError("EOF"))

		err = pubsub.Subscribe("mychannel")
		Expect(err).NotTo(HaveOccurred())
	})

	expectReceiveMessageOnError := func(pubsub *redis.PubSub) {
		pubsub.SetNetConn(&badConn{
			readErr:  io.EOF,
			writeErr: io.EOF,
		})

		step := make(chan struct{}, 3)

		go func() {
			defer GinkgoRecover()

			Eventually(step).Should(Receive())
			err := client.Publish("mychannel", "hello").Err()
			Expect(err).NotTo(HaveOccurred())
			step <- struct{}{}
		}()

		_, err := pubsub.ReceiveMessage()
		Expect(err).To(Equal(io.EOF))
		step <- struct{}{}

		msg, err := pubsub.ReceiveMessage()
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Channel).To(Equal("mychannel"))
		Expect(msg.Payload).To(Equal("hello"))

		Eventually(step).Should(Receive())
	}

	It("Subscribe should reconnect on ReceiveMessage error", func() {
		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		subscr, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(subscr).To(Equal(&redis.Subscription{
			Kind:    "subscribe",
			Channel: "mychannel",
			Count:   1,
		}))

		expectReceiveMessageOnError(pubsub)
	})

	It("PSubscribe should reconnect on ReceiveMessage error", func() {
		pubsub := client.PSubscribe("mychannel")
		defer pubsub.Close()

		subscr, err := pubsub.ReceiveTimeout(time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(subscr).To(Equal(&redis.Subscription{
			Kind:    "psubscribe",
			Channel: "mychannel",
			Count:   1,
		}))

		expectReceiveMessageOnError(pubsub)
	})

	It("should return on Close", func() {
		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer GinkgoRecover()

			wg.Done()
			defer wg.Done()

			_, err := pubsub.ReceiveMessage()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(SatisfyAny(
				Equal("redis: client is closed"),
				ContainSubstring("use of closed network connection"),
			))
		}()

		wg.Wait()
		wg.Add(1)

		Expect(pubsub.Close()).NotTo(HaveOccurred())

		wg.Wait()
	})

	It("should ReceiveMessage without a subscription", func() {
		timeout := 100 * time.Millisecond

		pubsub := client.Subscribe()
		defer pubsub.Close()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()

			time.Sleep(timeout)

			err := pubsub.Subscribe("mychannel")
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(timeout)

			err = client.Publish("mychannel", "hello").Err()
			Expect(err).NotTo(HaveOccurred())
		}()

		msg, err := pubsub.ReceiveMessage()
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Channel).To(Equal("mychannel"))
		Expect(msg.Payload).To(Equal("hello"))

		wg.Wait()
	})

	It("handles big message payload", func() {
		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		ch := pubsub.Channel()

		bigVal := bigVal()
		err := client.Publish("mychannel", bigVal).Err()
		Expect(err).NotTo(HaveOccurred())

		var msg *redis.Message
		Eventually(ch).Should(Receive(&msg))
		Expect(msg.Channel).To(Equal("mychannel"))
		Expect(msg.Payload).To(Equal(string(bigVal)))
	})

	It("supports concurrent Ping and Receive", func() {
		const N = 100

		pubsub := client.Subscribe("mychannel")
		defer pubsub.Close()

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()

			for i := 0; i < N; i++ {
				_, err := pubsub.ReceiveTimeout(5 * time.Second)
				Expect(err).NotTo(HaveOccurred())
			}
			close(done)
		}()

		for i := 0; i < N; i++ {
			err := pubsub.Ping()
			Expect(err).NotTo(HaveOccurred())
		}

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			Fail("timeout")
		}
	})
})
