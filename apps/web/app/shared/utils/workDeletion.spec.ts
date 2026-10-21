import { describe, expect, it } from 'vitest'
import { workDeletionReasonText } from './workDeletion'

describe('workDeletionReasonText', () => {
  it('names every reason catalog may give', () => {
    for (const reason of [
      'not_a_work',
      'out_of_scope',
      'spam_or_test',
      'other'
    ]) {
      expect(workDeletionReasonText(reason)).not.toBe('')
    }
  })

  it('shows no reason rather than a raw token it does not know', () => {
    expect(workDeletionReasonText('duplicate')).toBe('')
    expect(workDeletionReasonText('')).toBe('')
  })
})
