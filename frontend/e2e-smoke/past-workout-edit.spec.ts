import { expect, test, type Page } from "@playwright/test";
import { BARBELL_SQUAT, BENCH_PRESS, apiResponse, chooseExercise, dismissMonthlyPrompt, expectNoAICalls, localDateKey, registerUser } from "./support";

const editorHeading = (page: Page) => page.getByRole("heading", { name: "記録を編集" });

/** カレンダーを開いて前月に移動し、指定日の記録エディタを開く。日付の記録の取得結果を返す。 */
async function openDayInPreviousMonth(page: Page, date: string) {
  await page.goto("/calendar");
  await expect(page.getByRole("heading", { level: 1, name: "カレンダー" })).toBeVisible();
  await page.getByRole("button", { name: "前の月" }).click();
  const loaded = page.waitForResponse(apiResponse("GET", `/api/workouts/by-date/${date}`));
  await page.getByRole("button", { name: `${date} の記録を編集` }).click();
  const response = await loaded;
  await expect(editorHeading(page)).toBeVisible();
  return response;
}

async function saveDay(page: Page, date: string) {
  const saved = page.waitForResponse(apiResponse("PUT", `/api/workouts/by-date/${date}`));
  await page.getByRole("button", { name: "保存", exact: true }).click();
  const response = await saved;
  expect(response.status()).toBe(200);
  await expect(editorHeading(page)).toBeHidden();
  return response;
}

test("カレンダーから過去日の記録を作成・編集でき、再読み込み後も保存した内容が表示される", async ({ page, request }) => {
  await registerUser(page, "past");
  await dismissMonthlyPrompt(page);
  // 前月15日は実行日にかかわらず過去日で、どの月にも存在する。
  const date = await localDateKey(page, { monthOffset: -1, day: 15 });
  const title = "過去日のスモーク記録";
  const titleInput = page.getByPlaceholder("例: 胸の日", { exact: true });
  const weights = page.getByLabel("重量 kg");
  const reps = page.getByLabel("回数");
  const dayCell = page.getByRole("button", { name: `${date} の記録を編集` });

  const empty = await openDayInPreviousMonth(page, date);
  expect(empty.status(), "記録のない過去日は404から新規作成する").toBe(404);
  await chooseExercise(page, page.getByRole("button", { name: "種目を追加して記録を作成" }), BENCH_PRESS);
  await chooseExercise(page, page.getByRole("button", { name: "種目", exact: true }), BARBELL_SQUAT);
  await titleInput.fill(title);
  await expect(weights).toHaveCount(2);
  await weights.nth(0).fill("50");
  await reps.nth(0).fill("10");
  await weights.nth(1).fill("70");
  await reps.nth(1).fill("5");

  const created = await saveDay(page, date);
  expect(created.request().postDataJSON()).toEqual({
    title,
    sets: [
      { exercise_id: BENCH_PRESS.id, weight: 50, reps: 10, set_order: 1, feeling: "" },
      { exercise_id: BARBELL_SQUAT.id, weight: 70, reps: 5, set_order: 2, feeling: "" },
    ],
  });
  const createdBody = await created.json();
  expect(createdBody).toMatchObject({
    date,
    title,
    sets: [
      { exercise_id: BENCH_PRESS.id, exercise_name: BENCH_PRESS.name, weight: 50, reps: 10, set_order: 1 },
      { exercise_id: BARBELL_SQUAT.id, exercise_name: BARBELL_SQUAT.name, weight: 70, reps: 5, set_order: 2 },
    ],
  });
  const workoutID: number = createdBody.workout_id;
  expect(workoutID).toBeGreaterThan(0);
  await expect(dayCell).toContainText(title);

  // 保存済みの過去日を開き直して、重量を直し、1セットを削除する。
  const reopened = page.waitForResponse(apiResponse("GET", `/api/workouts/by-date/${date}`));
  await dayCell.click();
  expect((await reopened).status()).toBe(200);
  await expect(editorHeading(page)).toBeVisible();
  await expect(titleInput).toHaveValue(title);
  await expect(weights).toHaveCount(2);
  await expect(weights.nth(0)).toHaveValue("50");
  await weights.nth(0).fill("55");
  await page.getByRole("button", { name: "セットを削除" }).nth(1).click();
  await expect(weights).toHaveCount(1);

  const edited = await saveDay(page, date);
  expect(edited.request().postDataJSON()).toEqual({
    title,
    sets: [{ exercise_id: BENCH_PRESS.id, weight: 55, reps: 10, set_order: 1, feeling: "" }],
  });
  expect(await edited.json()).toMatchObject({
    workout_id: workoutID,
    date,
    title,
    sets: [{ exercise_id: BENCH_PRESS.id, weight: 55, reps: 10, set_order: 1 }],
  });

  // ページを読み込み直しても、サーバーに保存した編集後の内容が表示される。
  const persisted = await openDayInPreviousMonth(page, date);
  expect(persisted.status()).toBe(200);
  await expect(titleInput).toHaveValue(title);
  await expect(weights).toHaveCount(1);
  await expect(weights.nth(0)).toHaveValue("55");
  await expect(reps.nth(0)).toHaveValue("10");
  await expect(page.getByText(BENCH_PRESS.name, { exact: true })).toBeVisible();

  // 履歴の詳細にも編集後の集計が反映される。
  await page.getByRole("link", { name: "詳細" }).click();
  await expect(page).toHaveURL(new RegExp(`/workouts/${workoutID}$`));
  await expect(page.getByRole("heading", { level: 1, name: title, exact: true })).toBeVisible();
  await expect(page.getByText("550", { exact: true })).toBeVisible();

  await expectNoAICalls(request);
});
