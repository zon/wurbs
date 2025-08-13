<script setup lang="ts">
import { authUser, renameAuthUser } from 'gonf'
import { BadRequestError } from 'gonf'
import { User } from 'gonf'
import { router } from '../../router'
import { onMounted, ref, watch, type Ref } from 'vue'

const user = ref(new User())
const error: Ref<BadRequestError | null> = ref(null)

onMounted(() => {
  user.value = authUser.value
  if (!user.value.ready) {
    user.value.name = ''
  }
})

watch(authUser, (newUser) => {
  user.value = newUser
})

async function onSubmit() {
  try {
    await renameAuthUser(user.value.name)
  } catch (err) {
    if (err instanceof BadRequestError) {
      error.value = err
      return
    }
    error.value = null
    throw err
  }
  router.push('/')
}

</script>

<template>
  <div id="page">
    <h1 id="title">Wurbs!</h1>
    <div v-if="!user.isEmpty()" id="content">
      <h2 v-if="user.ready">Edit User #{{ user.id }}</h2>
      <div v-else>
        <h2>Welcome</h2>
        <p>Set your name. <span class="note">Can be changed at any time</span></p>
      </div>
      <form @submit.prevent="onSubmit">
        <div class="field">
          <label for="name">Name</label>
          <input id="name" name="name" type="text" v-model="user.name" />
        </div>
        <div v-if="error" id="error">
          <p>{{ error.message }}</p>
        </div>
        <div class="actions">
          <button v-if="user.ready" class="primary" type="submit">Save</button>
          <RouterLink v-if="user.ready" class="button" to="/">Cancel</RouterLink>
          <button v-else class="primary" type="submit">Set</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
  #page {
    margin: auto;
    max-width: 500px;
  }
  #content {
    border: 1px solid hsl(0 0% 30%);
    padding: 1em 2ex;
  }
  #content > *:first-child {
    margin-top: 0;
  }
  #content > *:last-child {
    margin-bottom: 0;
  }
</style>
