Create a Golang program that receives a string from an MQTT topic (see this https://randomnerdtutorials.com/what-is-mqtt-and-how-it-works/) and stores the received data in a Redis sorted set.

Then, use Golang Mux to create an api which returns all the stored data from redis
