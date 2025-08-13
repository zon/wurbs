<script setup lang="ts">
import { computed, onMounted, onUnmounted, useTemplateRef } from 'vue'
import type { Message } from '../models/Message'
import { formatDate } from 'gonf'

const props = defineProps<{
  message: Message
  observer: IntersectionObserver
}>()

const id = computed(() => `message-${props.message.id}`)
const time = computed(() => formatDate(props.message.createdAt))
const user = props.message.getUser()
const element = useTemplateRef('main')

onMounted(() => {
  if (element.value !== null) {
    props.observer.observe(element.value)
  }
})

onUnmounted(() => {
  if (element.value !== null) {
    props.observer.unobserve(element.value)
  }
})

</script>

<template>
  <div ref="main" :id class="message" :data-id="props.message.id">
    <p class="details">
      <span class="user">{{ user.name }}</span> <span class="time">{{ time }}</span>
    </p>
    <div class="content" v-html="message.content"></div>
  </div> 
</template>

<style scoped>
  .message .details {
    color: hsl(0, 0%, 50%);
    margin: 0;
  }
  .message .details .user {
    color: hsl(60 90% 75%);
  }
  .message .content > *:first-child {
    margin-top: 0;
  }
  .message .content > *:last-child {
    margin-bottom: 0;
  }
  .message .content pre {
    background-color: hsl(0 0% 10%);
    padding: 1em 2ex;
    overflow-x: scroll;
  }
  .message .content code {
    background-color: hsl(0 0% 10%);
  }
</style>
