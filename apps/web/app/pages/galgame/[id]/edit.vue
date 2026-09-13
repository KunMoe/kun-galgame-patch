<script setup lang="ts">
import type { EditProposal, EditSchemaField } from '@nextmoe/edit-ui-vue'
import { parseEditProblem } from '@nextmoe/edit-ui-vue'
import {
  CATALOG_EDIT_CONFIG,
  CATALOG_EDIT_FIELD,
  catalogEditFieldLabel,
  toCatalogEditRequest,
  type CatalogEditRequest,
  type CatalogEditTitle
} from '~/shared/utils/catalogEdit'
import { kunMoyuMoe } from '~/config/moyu-moe'

interface EditBootstrap {
  work_id: number
  can_edit: boolean
  values: {
    display_name: string
    olang: string
    content_rating: number
    titles: CatalogEditTitle[]
  }
  fields: EditSchemaField[]
}

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const id = Number(route.params.id)

useKunDisableSeo('编辑游戏资料')

const kungalOrigin = kunMoyuMoe.domain.kungal

const { data } = await useAsyncData(`patch-catalog-edit-${id}`, async () => {
  if (!userStore.isLoggedIn) return null
  return await api.get<EditBootstrap>(`/patch/${id}/catalog-edit`)
})

const denied = computed(() => {
  const code = data.value?.code
  if (!code) return ''
  if (code === 40400) return '资料库里没有这个条目，暂时无法编辑。'
  if (code === 50320) return '资料库服务暂不可用，请稍后再试。'
  return data.value?.message || '暂时无法编辑该游戏的资料。'
})

const loaded = computed(() => (data.value?.code === 0 ? data.value.data : null))

const fields = computed(() => loaded.value?.fields ?? [])
const values = computed<Record<string, unknown>>(() => {
  const b = loaded.value
  if (!b) return {}
  return {
    [CATALOG_EDIT_FIELD.displayName]: b.values.display_name,
    [CATALOG_EDIT_FIELD.olang]: b.values.olang,
    [CATALOG_EDIT_FIELD.contentRating]: b.values.content_rating,
    [CATALOG_EDIT_FIELD.titles]: b.values.titles ?? []
  }
})

const patch = ref<Record<string, unknown>>({})
const note = ref('')

const isDirectEdit = computed(
  () =>
    fields.value.length > 0 &&
    fields.value.every((f) => f.locked || f.would_automerge)
)

const { data: propData, refresh: refreshProposals } = await useAsyncData(
  `patch-catalog-proposals-${id}`,
  async () => {
    if (!userStore.isLoggedIn) return null
    return await api.get<{ items: EditProposal[] }>(
      `/patch/${id}/catalog-edit/proposals`
    )
  }
)
const myProposals = computed(() =>
  propData.value?.code === 0 ? (propData.value.data.items ?? []) : []
)

const withdraw = async (pid: number) => {
  const res = await api.post(`/catalog-proposal/${pid}/withdraw`)
  if (res.code === 0) {
    useKunMessage('提案已撤回', 'success')
    await refreshProposals()
  } else {
    useKunMessage(res.message || '撤回失败', 'error')
  }
}

// Every row here is the viewer's own, so the card never has to look a uid up.
const proposer = computed(() => ({
  id: userStore.user.id,
  name: userStore.user.name,
  avatar: userStore.user.avatar
}))

const saving = ref(false)
const form = useTemplateRef('form')
const fieldErrors = ref<Record<string, string[]>>({})
const formErrors = ref<string[]>([])

const save = async () => {
  if (Object.keys(patch.value).length === 0) {
    useKunMessage('没有需要保存的修改', 'info')
    return
  }
  const payload: CatalogEditRequest = toCatalogEditRequest(patch.value)
  if (note.value) payload.note = note.value

  saving.value = true
  const res = await api.post<{ merged: boolean }>(
    `/patch/${id}/catalog-edit`,
    payload
  )
  saving.value = false
  if (res.code !== 0) {
    const problem = parseEditProblem(res.data)
    fieldErrors.value = problem.fields
    formErrors.value = problem.form.length
      ? problem.form
      : [res.message || '保存失败，请稍后再试']
    return
  }
  fieldErrors.value = {}
  formErrors.value = []
  useKunMessage(
    res.data.merged ? '已保存（生成一条修订记录）' : '已提交提案，等待审核',
    'success'
  )
  await Promise.all([
    refreshNuxtData(`patch-catalog-edit-${id}`),
    refreshProposals()
  ])
  form.value?.reset()
  note.value = ''
}
</script>

<template>
  <AuthRequired>
    <div class="space-y-6">
      <div class="space-y-1">
        <h2 class="text-xl font-bold">编辑游戏资料</h2>
        <p class="text-default-500 text-sm">
          这里修改的是
          <a
            :href="`${kungalOrigin}/galgame/${id}`"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary hover:underline"
          >
            鲲 Galgame 资料库
          </a>
          里的条目，保存后全站生效。
        </p>
      </div>

      <KunInfo
        v-if="denied"
        color="warning"
        variant="flat"
        :description="denied"
      />

      <template v-else-if="loaded">
        <KunInfo
          v-if="!loaded.can_edit"
          color="warning"
          variant="flat"
          description="你当前没有编辑这个条目的权限，可以先到主站参与编辑，或在评论区反馈需要修正的内容。"
        />

        <form v-else class="flex flex-col gap-6" @submit.prevent="save">
          <p class="text-default-500 text-sm">
            {{
              isDirectEdit
                ? '你的修改会立即生效，并生成一条修订记录。'
                : '保存后会生成一份编辑提案，由资料库维护者审核后合并。'
            }}
          </p>

          <EditSchemaForm
            ref="form"
            :fields="fields"
            :values="values"
            :config="CATALOG_EDIT_CONFIG"
            :errors="fieldErrors"
            :form-errors="formErrors"
            @update:patch="(next) => (patch = next)"
          />

          <KunTextarea
            v-model="note"
            label="修改说明（可选）"
            placeholder="说明修改的依据，便于审核与追溯"
            :rows="3"
          />

          <div class="flex items-center gap-3">
            <KunButton type="submit" color="primary" :loading="saving">
              {{ isDirectEdit ? '保存' : '提交提案' }}
            </KunButton>
            <KunButton
              type="button"
              variant="light"
              color="default"
              :href="`/patch/${id}/introduction`"
            >
              返回
            </KunButton>
          </div>
        </form>

        <section v-if="myProposals.length" class="space-y-3">
          <h3 class="text-lg font-semibold">我的提案</h3>
          <EditProposalCard
            v-for="p in myProposals"
            :key="p.id"
            :proposal="p"
            :label-for="catalogEditFieldLabel"
            :proposer="proposer"
          >
            <template #actions>
              <KunButton
                v-if="p.status === 'open'"
                size="sm"
                variant="flat"
                color="danger"
                type="button"
                @click="withdraw(p.id)"
              >
                撤回
              </KunButton>
            </template>
          </EditProposalCard>
        </section>

        <div
          class="border-default-200 flex flex-wrap items-center gap-x-4 gap-y-2 border-t pt-4 text-sm"
        >
          <a
            :href="`${kungalOrigin}/galgame/${id}`"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary flex items-center gap-1.5 hover:underline"
          >
            <KunIcon name="lucide:external-link" class="size-4" />
            前往主站完整编辑
          </a>
          <a
            :href="`${kungalOrigin}/galgame/${id}/history`"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary flex items-center gap-1.5 hover:underline"
          >
            <KunIcon name="lucide:history" class="size-4" />
            修订历史
          </a>
          <span class="text-default-400 text-xs">
            封面、截图、职员表等资料请到主站编辑。
          </span>
        </div>
      </template>

      <KunLoading v-else description="加载中..." />
    </div>
  </AuthRequired>
</template>
