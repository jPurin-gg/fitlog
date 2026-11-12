import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import BodyMeasurements from "./BodyMeasurements";

interface StoredMeasurement {
  id: number;
  measured_on: string;
  weight_kg: number;
  body_fat_percent?: number | null;
  resting_heart_rate?: number | null;
}

let stored: StoredMeasurement[] = [];
let nextId = 1;

const server = setupServer(
  http.get("http://localhost:3000/api/body-measurements", () => HttpResponse.json(stored)),
  http.post("http://localhost:3000/api/body-measurements", async ({ request }) => {
    const body = (await request.json()) as Omit<StoredMeasurement, "id">;
    const created = { ...body, id: nextId++ };
    stored = [created, ...stored];
    return HttpResponse.json(created, { status: 201 });
  }),
  http.delete("http://localhost:3000/api/body-measurements/:id", ({ params }) => {
    stored = stored.filter(item => item.id !== Number(params.id));
    return new HttpResponse(null, { status: 204 });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => { server.resetHandlers(); stored = []; nextId = 1; });
afterAll(() => server.close());

it("記録がなければ空状態と記録ボタンを表示する", async () => {
  render(<BodyMeasurements />);
  expect(await screen.findByText(/まだ身体データの記録はありません/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /記録する/ })).toBeInTheDocument();
});

it("体重を記録すると一覧に表示され、削除できる", async () => {
  const onChanged = vi.fn();
  render(<BodyMeasurements onChanged={onChanged} />);
  await screen.findByText(/まだ身体データの記録はありません/);

  await userEvent.click(screen.getByRole("button", { name: /記録する/ }));
  await userEvent.type(screen.getByLabelText(/体重/), "68.5");
  await userEvent.click(screen.getByRole("button", { name: "保存する" }));

  expect(await screen.findByText("68.5 kg")).toBeInTheDocument();
  expect(onChanged).toHaveBeenCalled();

  vi.spyOn(window, "confirm").mockReturnValue(true);
  await userEvent.click(screen.getByRole("button", { name: "削除" }));
  await waitFor(() => expect(screen.queryByText("68.5 kg")).not.toBeInTheDocument());
});
